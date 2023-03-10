package index

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/bufbuild/connect-go"
	api "github.com/srerickson/ocfl-index/gen/ocfl/v0"
)

const monMsgBuffLen = 64  // size for async monitor's buffered message channel
const monMaxSessions = 64 // max number of simultaneous connections to the async monitor

var (
	ErrAsyncNotReady           = errors.New("another task is running")
	ErrAsyncMonitorMaxSessions = errors.New("cannot accept additional monitoring sessions")
	ErrAsyncMonitorSend        = errors.New("failed to send message to monitoring session")
)

// Async is used to run asynchronous indexing tasks
type Async struct {
	status  string
	done    chan struct{}
	tokenCh chan asyncToken
	taskCh  chan asyncTask
	monitor monitor
}

func NewAsync(ctx context.Context) *Async {
	async := &Async{
		done:    make(chan struct{}),
		tokenCh: make(chan asyncToken, 1),
		taskCh:  make(chan asyncTask, 1),
	}
	async.monitor.Start()
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		async.workLoop(ctx)
	}()
	go func() {
		wg.Wait()
		close(async.tokenCh)
		close(async.taskCh)
		async.monitor.Close()
	}()
	return async
}

// Async's primary work look
func (sch *Async) workLoop(ctx context.Context) {
	for {
		var task asyncTask
		// block until task, closed, or canceled ctx
		select {
		case task = <-sch.taskCh:
		case <-sch.done:
			return
		case <-ctx.Done():
			close(sch.done)
			return
		}
		sch.status = task.Name
		if sch.status == "" {
			sch.status = "busy"
		}
		// modify the context
		// allow deadlines?
		if task.Fn != nil {
			task.run(ctx, &sch.monitor)
		}
		// ready for new task
		sch.status = "ready"
		<-sch.tokenCh
	}
}

// Close frees Async resources
func (sch *Async) Close() {
	close(sch.done)
}

// Wait blocks Async is fully shutdown
func (sch *Async) Wait() {
	<-sch.done
}

func (sch *Async) TryNow(name string, fn taskFn) error {
	task := asyncTask{Fn: fn, Name: name}
	select {
	case sch.tokenCh <- asyncToken{}:
		sch.taskCh <- task
		return nil
	default:
		return ErrAsyncNotReady
	}
}

func (sch *Async) MonitorOn(ctx context.Context, rq *connect.Request[api.ReindexRequest], stream *connect.ServerStream[api.ReindexResponse]) error {
	return sch.monitor.Handle(ctx, rq, stream)
}

func (sch *Async) Status() string {
	return sch.status
}

// asyncToken is used to signal a new task
type asyncToken struct{}

type taskFn func(context.Context, io.Writer) error

type asyncTask struct {
	Name  string
	Fn    taskFn
	Err   error
	Panic error
}

func (t *asyncTask) run(ctx context.Context, w io.Writer) {
	defer func() {
		if v := recover(); v != nil {
			if err, ok := v.(error); ok {
				t.Panic = err
				return
			}
			t.Panic = fmt.Errorf("panic: %v", v)
		}
	}()
	t.Err = t.Fn(ctx, w)
}

// monitor is an io.Writer that forwards messages to registered grpc sessions
type monitor struct {
	sessions   sessionMap                                // map of all connections
	msgCh      chan string                               // for new messages
	sessInitCh chan monitorRequest                       // channel for new session requests
	sessFreeCh chan *connect.Request[api.ReindexRequest] // channel for freeing resource on a session
	done       chan struct{}                             // to close the monitor
}

func (m *monitor) Start() {
	m.sessions = make(sessionMap)
	m.sessInitCh = make(chan monitorRequest, 1)
	m.sessFreeCh = make(chan *connect.Request[api.ReindexRequest], 1)
	m.msgCh = make(chan string, monMsgBuffLen)
	m.done = make(chan struct{})
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		m.runLoop()
	}()
	go func() {
		wg.Wait()
		close(m.sessInitCh)
		close(m.sessFreeCh)
		close(m.msgCh)
	}()
}

// Close closes the monitor, freeing resources associated with it.
func (m *monitor) Close() {
	close(m.done)
}

func (m *monitor) Write(b []byte) (int, error) {
	msg := strings.TrimRight(string(b), "\n")
	select {
	case m.msgCh <- msg:
		return len([]byte(msg)), nil
	case <-m.done:
		return 0, io.ErrClosedPipe
	}
}

func (m *monitor) Handle(ctx context.Context, rq *connect.Request[api.ReindexRequest], stream *connect.ServerStream[api.ReindexResponse]) error {
	errCh := make(chan error, 1)
	defer close(errCh)
	m.sessInitCh <- monitorRequest{rq, stream, errCh}
	for {
		select {
		case err := <-errCh:
			if err == nil {
				continue
			}
			if !errors.Is(err, ErrAsyncMonitorMaxSessions) {
				// we only need to free the session resources
				// if the session was successfully created in the
				// first place
				m.sessFreeCh <- rq
			}
			return err
		case <-ctx.Done():
			// connection closed by client
			m.sessFreeCh <- rq
			return ctx.Err()
		case <-m.done:
			// monitor shutting down
			return errors.New("server closed connection")
		}
	}
}

func (m *monitor) runLoop() {
	for {
		select {
		case s := <-m.sessInitCh:
			if len(m.sessions) >= monMaxSessions {
				s.errCh <- ErrAsyncMonitorMaxSessions
				break // from select
			}
			m.sessions[s.rq] = monitorSession{
				stream: s.stream,
				errCh:  s.errCh,
			}
		case r := <-m.sessFreeCh:
			delete(m.sessions, r)
		case msg := <-m.msgCh:
			for _, sess := range m.sessions {
				resp := &api.ReindexResponse{LogMessage: msg}
				if err := sess.stream.Send(resp); err != nil {
					sess.errCh <- ErrAsyncMonitorSend
				}
			}
		case <-m.done:
			// clear conns and break loop
			for r := range m.sessions {
				delete(m.sessions, r)
			}
			return
		}
	}
}

type sessionMap map[*connect.Request[api.ReindexRequest]]monitorSession

// request to establish a new session
type monitorRequest struct {
	rq     *connect.Request[api.ReindexRequest]
	stream *connect.ServerStream[api.ReindexResponse]
	errCh  chan error // error while establishing a session
}

// an established monitor session
type monitorSession struct {
	stream *connect.ServerStream[api.ReindexResponse]
	errCh  chan error
}

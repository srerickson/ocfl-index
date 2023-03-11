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

// Async's primary work loop runs with the server's background context
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
		// TODO: configurable deadlines for task
		task.run(ctx, &sch.monitor)
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

func (sch *Async) TryNow(name string, fn taskFn) (bool, chan error) {
	select {
	case sch.tokenCh <- asyncToken{}:
		errch := make(chan error, 1) // channel is closed during task run()
		sch.taskCh <- asyncTask{Fn: fn, Name: name, ErrCh: errch}
		return true, errch
	default:
		return false, nil
	}
}

func (sch *Async) MonitorOn(ctx context.Context, rq *connect.Request[api.ReindexRequest], stream *connect.ServerStream[api.ReindexResponse], errCh chan error) error {
	return sch.monitor.Handle(ctx, rq, stream, errCh)
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
	ErrCh chan error
	err   error
}

func (t *asyncTask) run(ctx context.Context, w io.Writer) {
	defer func() {
		if v := recover(); v != nil {
			panicErr, ok := v.(error)
			if !ok {
				panicErr = fmt.Errorf("panic: %v", v)
			}
			t.err = errors.Join(t.err, panicErr)
		}
		t.ErrCh <- t.err
		close(t.ErrCh)
	}()
	if t.Fn == nil {
		return
	}
	t.err = t.Fn(ctx, w)
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
	m.sessInitCh = make(chan monitorRequest)
	m.sessFreeCh = make(chan *connect.Request[api.ReindexRequest])
	m.msgCh = make(chan string, monMsgBuffLen)
	m.done = make(chan struct{}) // should be closed explicitly
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

// Handle registers a request/stream pair with the monitor causing monitor log
// message to be streamed to the client. It blocks until one of the following
// occurs:
//
// - a monitoring session cannot be established or the monitor encounters an
// error while sending messages to the stream.
//
// - the taskErr channel is closed (i.e., the associated task has run to
// completion)
//
// - The connection context (ctx) is canceled
//
// - the monitor is shutdown on the server side
//
// Note that taskErr may be nill, in which case the request will monitor any
// existing future tasks but it will never disconnect whey those tasks complete.
func (m *monitor) Handle(ctx context.Context, rq *connect.Request[api.ReindexRequest], stream *connect.ServerStream[api.ReindexResponse], taskErrCh chan error) error {
	monErrCh := make(chan error, 1) // used to receive error while establishing the monitiros session
	defer close(monErrCh)
	m.sessInitCh <- monitorRequest{rq, stream, monErrCh}
	for {
		select {
		case err := <-taskErrCh:
			// return value from task: end the session
			m.sessFreeCh <- rq
			return err
		case err := <-monErrCh:
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

// runLoop is the monitor's main loop. It listens for new monitor sessions,
// frees sessions resources and send messages to all existing sessions. It runs
// until the monitor's done channel is closed.
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

// session map is used by monitor to track current connections
type sessionMap map[*connect.Request[api.ReindexRequest]]monitorSession

// request to establish a new session
type monitorRequest struct {
	rq     *connect.Request[api.ReindexRequest]
	stream *connect.ServerStream[api.ReindexResponse]
	errCh  chan error // error establishing a session, or sending messages
}

// an established monitor session
type monitorSession struct {
	stream *connect.ServerStream[api.ReindexResponse]
	errCh  chan error // error establishing a session, or sending messages
}

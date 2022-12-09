package server

import (
	"errors"
	"fmt"
	"net/http"
)

var errMonitorClosed = errors.New("monitor service is not running")

// monitor receives messages via Write and sends them to requesters registered
// with HandleRequest as server-sent events. Use NewMonitor to create a new
// monitor.
type monitor struct {
	// map of all connections
	conns     map[*http.Request]http.ResponseWriter
	newMsg    chan string         // for new messages
	newConn   chan monitorRequest // for new connections
	closeConn chan *http.Request  // to close a connection
	close     chan struct{}       // to close the monitor
}

type monitorRequest struct {
	r *http.Request
	w http.ResponseWriter
}

// NewMonitor initializes a new monitor and returns a pointer. Use
// Close to free resources assocaited with the monitor.
func NewMonitor() *monitor {
	m := &monitor{
		conns:     make(map[*http.Request]http.ResponseWriter),
		newConn:   make(chan monitorRequest),
		closeConn: make(chan *http.Request),
		newMsg:    make(chan string),
		close:     make(chan struct{}),
	}
	go m.run()
	return m
}

// run is the monitor's message processing loop. It runs
// until the monitor's close channel is closed
func (m *monitor) run() {
	for {
		select {
		case c := <-m.newConn:
			m.conns[c.r] = c.w
		case r := <-m.closeConn:
			delete(m.conns, r)
		case msg := <-m.newMsg:
			for _, w := range m.conns {
				// FIXME no error handling here
				if _, err := fmt.Fprint(w, msg); err == nil {
					w.(http.Flusher).Flush()
				}
			}
		case <-m.close:
			// clear conns and break loop
			for r := range m.conns {
				delete(m.conns, r)
			}
			return
		}
	}
}

func (m *monitor) Write(b []byte) (int, error) {
	msg := fmt.Sprintf("data: %s\n\n", string(b))
	if m.newMsg == nil || m.close == nil {
		return 0, errMonitorClosed
	}
	select {
	case m.newMsg <- msg:
		return len(b), nil
	case <-m.close:
		return 0, errMonitorClosed
	}
}

func (m *monitor) HandleRequest(w http.ResponseWriter, r *http.Request) {
	if _, ok := w.(http.Flusher); !ok {
		http.Error(w, "streaming not supported", http.StatusExpectationFailed)
		return
	}
	if m.newConn == nil {
		http.Error(w, errMonitorClosed.Error(), http.StatusInternalServerError)
		return
	}
	select {
	case <-m.close:
		http.Error(w, errMonitorClosed.Error(), http.StatusExpectationFailed)
	default:
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	m.newConn <- monitorRequest{r, w}
	for {
		select {
		case <-r.Context().Done():
			// connection closed by context
			m.closeConn <- r
			return
		case <-m.close:
			// monitor closed on server side
			http.Error(w, errMonitorClosed.Error(), http.StatusExpectationFailed)
			return
		}
	}
}

// Close closes the monitor, freeing resources associated with it.
func (m *monitor) Close() {
	close(m.close)
	close(m.newConn)
	close(m.newMsg)
	close(m.closeConn)
}

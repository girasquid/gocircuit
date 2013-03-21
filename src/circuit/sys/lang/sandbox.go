package lang

import (
	"circuit/use/circuit"
	"encoding/gob"
	"io"
	"sync"
)

type sandbox struct {
	lk sync.Mutex
	l  map[circuit.WorkerID]*listener
}

var s = &sandbox{l: make(map[circuit.WorkerID]*listener)}

// NewSandbox creates a new transport instance, part of a sandbox network in memory
func NewSandbox() circuit.Transport {
	s.lk.Lock()
	defer s.lk.Unlock()

	l := &listener{
		id: circuit.ChooseWorkerID(),
		ch: make(chan *halfconn),
	}
	l.a = &addr{ID: l.id, l: l}
	s.l[l.id] = l
	return l
}

func dial(remote circuit.Addr) (circuit.Conn, error) {
	pr, pw := io.Pipe()
	qr, qw := io.Pipe()
	srvhalf := &halfconn{PipeWriter: qw, PipeReader: pr}
	clihalf := &halfconn{PipeWriter: pw, PipeReader: qr}
	s.lk.Lock()
	l := s.l[remote.(*addr).WorkerID()]
	s.lk.Unlock()
	if l == nil {
		panic("unknown listener id")
	}
	go func() {
		l.ch <- srvhalf
	}()
	return ReadWriterConn(l.Addr(), clihalf), nil
}

// addr implements Addr
type addr struct {
	ID circuit.WorkerID
	l  *listener
}

func (a *addr) Host() string {
	panic("no physical underlying host")
}

func (a *addr) WorkerID() circuit.WorkerID {
	return a.ID
}

func (a *addr) String() string {
	return a.ID.String()
}

func init() {
	gob.Register(&addr{})
}

// listener implements Listener
type listener struct {
	id circuit.WorkerID
	a  *addr
	ch chan *halfconn
}

func (l *listener) Addr() circuit.Addr {
	return l.a
}

func (l *listener) Accept() circuit.Conn {
	return ReadWriterConn(l.Addr(), <-l.ch)
}

func (l *listener) Close() {
	s.lk.Lock()
	defer s.lk.Unlock()
	delete(s.l, l.id)
}

func (l *listener) Dial(remote circuit.Addr) (circuit.Conn, error) {
	return dial(remote)
}

// halfconn is one end of a byte-level connection
type halfconn struct {
	*io.PipeReader
	*io.PipeWriter
}

func (h *halfconn) Close() error {
	return h.PipeWriter.Close()
}

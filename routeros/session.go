package routeros

import (
	"errors"
	"sync"
)

// Conn is one live API connection.
type Conn interface {
	Commander
	Close() error
}

// Session is a Commander that dials on first use and dials again after the
// connection breaks. A command that fails because the connection broke returns
// that error and is NEVER retried; the next command dials again. A DeviceError
// keeps the connection. Safe for concurrent use: commands are serialized.
type Session struct {
	mu         sync.Mutex
	dial       func() (Conn, error)
	conn       Conn
	generation uint64
}

// NewSession creates a new reconnecting Session.
func NewSession(dial func() (Conn, error)) *Session {
	return &Session{
		dial: dial,
	}
}

// Run executes a command over the session, dialing if necessary.
func (s *Session) Run(sentence ...string) (Reply, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.conn == nil {
		conn, err := s.dial()
		if err != nil {
			return Reply{}, err
		}
		s.conn = conn
		s.generation++
	}

	reply, err := s.conn.Run(sentence...)
	if err != nil {
		var devErr DeviceError
		if !errors.As(err, &devErr) {
			s.conn.Close()
			s.conn = nil
		}
		return Reply{}, err
	}

	return reply, nil
}

// Generation counts successful dials; it changes when a new connection was made.
func (s *Session) Generation() uint64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.generation
}

// Close closes the underlying connection.
func (s *Session) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.conn != nil {
		err := s.conn.Close()
		s.conn = nil
		return err
	}
	return nil
}

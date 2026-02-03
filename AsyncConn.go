package osfp

import (
	"fmt"
	"net"
	"os"
	"sync"
	"syscall"
	"time"
)

var ERR_CANNOT_SET_DEADLINE_IN_THE_PAST = fmt.Errorf("Cannot set a deadline in the past")
var ERR_CANNOT_BE_SET_OUTSIDE_OF_EVENT_LOOP = fmt.Errorf("Operation cannot be performed outside of an event loop")

type AsyncConn struct {
	Job     *SockeStreamtJob
	Event   *StreamEvent
	Local   net.Addr
	Remote  net.Addr
	Err     error
	Lock    sync.RWMutex
	In      []byte
	Out     []byte
	running bool
	closed  bool
}

// This mehtod never blocks, it simply returns the last chunk of data
// in the internal buffer.
func (s *AsyncConn) Read(b []byte) (n int, err error) {
	s.Lock.Lock()
	defer s.Lock.Unlock()
	if s.Err != nil {
		err = s.Err
		return
	}
	size := len(s.In)
	if size == 0 {
		err = syscall.EAGAIN
		return
	}
	limit := len(b)
	if size > limit {
		n = limit
		copy(b, s.In[0:limit])
		s.In = s.In[limit:]
	} else {
		n = size
		copy(b, s.In[0:limit])
	}
	return
}

func (s *AsyncConn) ToggleRunning(running bool) {
	s.Lock.Lock()
	defer s.Lock.Unlock()
	s.running = running
}

// Write writes data to the connection.
func (s *AsyncConn) Write(b []byte) (n int, err error) {
	s.Lock.Lock()
	defer s.Lock.Unlock()
	if s.Err != nil {
		err = s.Err
		return
	}
	n = len(b)
	s.Out = append(s.Out, b...)

	return
}

// Close closes the connection.
// Any blocked Read or Write operations will be unblocked and return errors.
// Close may or may not block until any buffered data is sent;
// for TCP connections see [*TCPConn.SetLinger].
func (s *AsyncConn) Close() error {
	s.Lock.Lock()
	defer s.Lock.Unlock()
	if s.Err != nil {
		return s.Err
	}

	if s.closed {
		return os.ErrClosed
	}
	s.Err = os.ErrClosed
	s.closed = true
	if s.running {
		s.Event.Release()
	} else {
		s.Job.Release()
	}
	return nil
}

// LocalAddr returns the local network address, if known.
func (s *AsyncConn) LocalAddr() net.Addr {
	return s.Local
}

// RemoteAddr returns the remote network address, if known.
func (s *AsyncConn) RemoteAddr() net.Addr {
	return s.Remote
}

// SetDeadline sets the read and write deadlines associated
// with the connection. It is equivalent to calling both
// SetReadDeadline and SetWriteDeadline.
//
// # Notes
//
// In this implementation both read and write timeouts are shared.
// So calling SetReadDeadline or SetWriteDeadline is the same as
// making a call to this method.
//
// Setting a timeout in the past will trigger the following error
//   - osfp.ERR_CANNOT_SET_DEADLINE_IN_THE_PAST
func (s *AsyncConn) SetDeadline(t time.Time) (e error) {
	s.Lock.RLock()
	defer s.Lock.RUnlock()
	if s.Err != nil {
		e = s.Err
		return
	}

	var timeout int64
	if t.IsZero() {
		timeout = 0
	} else {
		now := time.Now().UnixMilli()
		diff := t.UnixMilli() - now
		if diff < 0 {
			e = ERR_CANNOT_SET_DEADLINE_IN_THE_PAST
			s.Err = e
		}
		if diff == 0 {
			// Internals do not support 0, wil have to wait 1 ms
			timeout = 1
		} else {
			timeout = diff
		}
	}
	if s.running {
		s.Event.SetTimeout(timeout)
	} else {
		s.Job.SetTimeout(timeout)
	}

	return
}

// This is just a wrapper for SetDeadline.
func (s *AsyncConn) SetReadDeadline(t time.Time) error {
	return s.SetDeadline(t)
}

// This is just a wrapper for SetDeadline.
func (s *AsyncConn) SetWriteDeadline(t time.Time) error {
	return s.SetDeadline(t)
}

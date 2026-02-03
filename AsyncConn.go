package osfp

import (
	"fmt"
	"net"
	"os"
	"time"
)

var ERR_CANNOT_SET_DEADLINE_IN_THE_PAST = fmt.Errorf("Cannot set a deadline in the past")

type AsyncConn struct {
	File   *os.File
	Job    *SockeStreamtJob
	Local  net.Addr
	Remote net.Addr
	Err    error
}

// Read reads data from the connection.
// Read can be made to time out and return an error after a fixed
// time limit; see SetDeadline and SetReadDeadline.
func (s *AsyncConn) Read(b []byte) (n int, err error) {
	s.Job.Lock.RLock()
	defer s.Job.Lock.RUnlock()
	if s.Err != nil {
		return -1, s.Err
	}
	return s.File.Read(b)
}

// Write writes data to the connection.
// Write can be made to time out and return an error after a fixed
// time limit; see SetDeadline and SetWriteDeadline.
func (s *AsyncConn) Write(b []byte) (n int, err error) {
	s.Job.Lock.RLock()
	defer s.Job.Lock.RUnlock()
	if s.Err != nil {
		return -1, s.Err
	}
	return s.File.Write(b)
}

// Close closes the connection.
// Any blocked Read or Write operations will be unblocked and return errors.
// Close may or may not block until any buffered data is sent;
// for TCP connections see [*TCPConn.SetLinger].
func (s *AsyncConn) Close() error {
	s.Job.Release()
	s.Job.Lock.Lock()
	defer s.Job.Lock.Unlock()
	return s.File.Close()
}

// LocalAddr returns the local network address, if known.
func (s *AsyncConn) LocalAddr() net.Addr {
	return s.Local
}

// RemoteAddr returns the remote network address, if known.
func (s *AsyncConn) RemoteAddr() net.Addr {
	s.Job.Lock.RLock()
	defer s.Job.Lock.RUnlock()
	return s.Remote
}

// SetDeadline sets the read and write deadlines associated
// with the connection. It is equivalent to calling both
// SetReadDeadline and SetWriteDeadline.
//
// A deadline is an absolute time after which I/O operations
// fail instead of blocking. The deadline applies to all future
// and pending I/O, not just the immediately following call to
// Read or Write. After a deadline has been exceeded, the
// connection can be refreshed by setting a deadline in the future.
//
// If the deadline is exceeded a call to Read or Write or to other
// I/O methods will return an error that wraps os.ErrDeadlineExceeded.
// This can be tested using errors.Is(err, os.ErrDeadlineExceeded).
// The error's Timeout method will return true, but note that there
// are other possible errors for which the Timeout method will
// return true even if the deadline has not been exceeded.
//
// An idle timeout can be implemented by repeatedly extending
// the deadline after successful Read or Write calls.
//
// A zero value for t means I/O operations will not time out.
func (s *AsyncConn) SetDeadline(t time.Time) (e error) {
	s.Job.Lock.Lock()
	defer s.Job.Lock.RUnlock()
	if s.Err != nil {
		e = s.Err
		return
	}

	if t.IsZero() {
		s.Job.UnsafeSetTimeout(0)
	} else {
		now := time.Now().UnixMilli()
		diff := t.UnixMilli() - now
		if diff < 0 {
			e = ERR_CANNOT_SET_DEADLINE_IN_THE_PAST
			s.Err = e
			return
		}
		if diff == 0 {
			// Internals do not support 0, wil have to wait 1 ms
			s.Job.SetTimeout(1)
		} else {
			s.Job.SetTimeout(diff)
		}

	}
	return
}

func (s *AsyncConn) GetError() error {
	s.Job.Lock.RLock()
	defer s.Job.Lock.RUnlock()
	return s.Err
}

// SetReadDeadline sets the deadline for future Read calls
// and any currently-blocked Read call.
// A zero value for t means Read will not time out.
func (s *AsyncConn) SetReadDeadline(t time.Time) error {
	return s.SetDeadline(t)
}

// SetWriteDeadline sets the deadline for future Write calls
// and any currently-blocked Write call.
// Even if write times out, it may return n > 0, indicating that
// some of the data was successfully written.
// A zero value for t means Write will not time out.
func (s *AsyncConn) SetWriteDeadline(t time.Time) error {
	return s.SetDeadline(t)
}

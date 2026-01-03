package osfp

import (
	"os"
	"sync"
)

type CallBackJob struct {
	Timeout int64
	Events  int16
	OnEvent func(curentEvents int16, err error)
	Worker  *Worker
	Fd      int32
	lock    sync.RWMutex
}

func NewJobFromFdT(fd int32, watchEvents int16, timeout int64, cb func(int16, error)) (job *CallBackJob) {
	job = &CallBackJob{
		OnEvent: cb,
		Timeout: timeout,
		Fd:      fd,
		Events:  watchEvents,
	}
	return
}

func NewJobFromOsFileT(f os.File, watchEvents int16, timeout int64, cb func(int16, error)) *CallBackJob {
	return NewJobFromFdT(int32(f.Fd()), watchEvents, timeout, cb)
}

// Updates the current timeout.
func (s *CallBackJob) SetTimeout(t int64) {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.Timeout = t
	if s.Worker != nil {
		s.Worker.Wakeup()
	}
}

// Processes the last epoll events and returns the next flags to use.
func (s *CallBackJob) ProcessEvents(currentEvents int16, now int64) (watchEevents int16, futureTimeOut int64, EventError error) {

	s.lock.RLock()
	defer s.lock.RUnlock()
	switch {
	case currentEvents&CAN_RW != 0 && s.OnEvent != nil:
		s.OnEvent(currentEvents, nil)
	case s.Timeout != 0:
		futureTimeOut = now + s.Timeout
	}
	watchEevents = s.Events

	return
}

// Called to validate the "lastTimeout", should return a futureTimeOut or 0 if there is no timeout.
// If the Job has timed out TimeOutError should be set to os.ErrDeadlineExceeded.
func (s *CallBackJob) CheckTimeOut(now int64, lastTimeout int64) (futureTimeOut int64, TimeOutError error) {
	s.lock.RLock()
	defer s.lock.RUnlock()
	if s.Timeout == 0 {
		return
	}
	futureTimeOut = lastTimeout
	// if we got here.. we need to make sure the old timeout isn't bad
	if lastTimeout < now {
		TimeOutError = os.ErrDeadlineExceeded
	}

	return
}

// Sets the current Worker. This method is called when a Job is added to a Worker in the pool.
func (s *CallBackJob) SetPool(worker *Worker, now int64) (watchEevents int16, futureTimeOut int64, fd int32) {
	s.lock.Lock()
	defer s.lock.Unlock()
	if s.Timeout != 0 {
		futureTimeOut = now + s.Timeout
	}
	watchEevents = s.Events
	fd = s.Fd

	return
}

// This is called when Job is being removed from the pool.
// Make sure to remove the refernce of the current worker when implementing this method.
// The error value is nil if the "watchEvents" value is 0 and no errors were found.
func (s *CallBackJob) ClearPool(e error) {

	s.Worker = nil
	if s.OnEvent != nil {
		s.OnEvent(0, e)
	}
}

package osfp

import (
	"errors"
	"os"
	"sync"
)

var ERR_CALLBACK_PANIC = errors.New("Callback Panic")

type CallBackJob struct {
	timeout int64
	events  uint32
	onEvent func(event *CallbackEvent)
	worker  *Worker
	fd      int32
	Lock    sync.RWMutex
	ran     bool
	jobId   int64
}

func NewJobFromFdT(fd int32, watchEvents uint32, timeout int64, cb func(*CallbackEvent)) (job *CallBackJob) {
	job = &CallBackJob{
		onEvent: cb,
		timeout: timeout,
		fd:      fd,
		events:  watchEvents,
		jobId:   nextJobId(),
	}

	return
}

func NewJobFromOsFileT(f *os.File, watchEvents uint32, timeout int64, cb func(*CallbackEvent)) *CallBackJob {
	return NewJobFromFdT(int32(f.Fd()), watchEvents, timeout, cb)
}

// Updates the current timeout.
func (s *CallBackJob) SetTimeout(t int64) {
	s.Lock.Lock()
	defer s.Lock.Unlock()
	if t == s.timeout {
		// nothing to see here.. move along
		return
	}
	s.timeout = t
	if s.worker != nil {
		s.worker.pushJobConfig(s.jobId)
	}
}

// Processes the last epoll events and returns the next flags to use.
func (s *CallBackJob) ProcessEvents(currentEvents uint32, now int64) (watchEevents uint32, futureTimeOut int64, EventError error) {

	s.Lock.RLock()
	defer s.Lock.RUnlock()
	s.ran = false
	if currentEvents&CAN_RW != 0 {
		config := &CallbackEvent{
			timeout:       s.timeout,
			nextTs:        -1,
			events:        s.events,
			currentEvents: currentEvents,
			now:           s.worker.now,
		}

		s.safeEvent(config)
		EventError = config.Error()

	}
	if s.timeout != 0 {
		futureTimeOut = now + s.timeout
	}
	watchEevents = s.events

	return
}

func (s *CallBackJob) safeEvent(config *CallbackEvent) {
	defer s.onRecover(config)
	if s.ran {
		return
	}
	s.ran = true
	if s.onEvent != nil {
		s.onEvent(config)
	}
	s.events = config.events
	s.timeout = config.timeout
}

// Called to validate the "lastTimeout", should return a futureTimeOut or 0 if there is no timeout.
// If the Job has timed out TimeOutError should be set to os.ErrDeadlineExceeded.
func (s *CallBackJob) CheckTimeOut(now int64, lastTimeout int64) (NewEvents uint32, futureTimeOut int64, TimeOutError error) {
	s.Lock.RLock()
	defer s.Lock.RUnlock()
	s.ran = false
	if s.timeout == 0 {
		futureTimeOut = 0
		NewEvents = s.events
		return
	}
	futureTimeOut = lastTimeout
	NewEvents = s.events
	if lastTimeout == 0 {
		futureTimeOut = now + s.timeout

	} else if lastTimeout <= now {

		// if we got here.. we need to make sure the old timeout isn't bad
		res := &CallbackEvent{
			events:  s.events,
			timeout: s.timeout,
			error:   os.ErrDeadlineExceeded,
			now:     s.worker.now,
		}
		s.safeEvent(res)
		NewEvents = res.events
		TimeOutError = res.error
		if res.error == nil {
			if s.timeout != 0 {
				futureTimeOut = now + s.timeout
			} else if s.events == 0 {
				TimeOutError = os.ErrDeadlineExceeded
			}
		}

	}

	return
}

// Sets the current Worker. This method is called when a Job is added to a Worker in the pool.
func (s *CallBackJob) SetPool(worker *Worker, now int64) (watchEevents uint32, futureTimeOut int64, fd int32) {
	s.Lock.Lock()
	defer s.Lock.Unlock()
	s.ran = false
	if s.timeout != 0 {
		futureTimeOut = now + s.timeout

	}
	watchEevents = s.events
	fd = s.fd
	s.worker = worker

	return
}

// This is called when Job is being removed from the pool.
// Make sure to remove the refernce of the current worker when implementing this method.
// The error value is nil if the "watchEvents" value is 0 and no errors were found.
func (s *CallBackJob) ClearPool(e error) {
	s.Lock.Lock()
	defer s.Lock.Unlock()
	s.worker = nil
	s.safeEvent(&CallbackEvent{error: e})
}

func (s *CallBackJob) onRecover(config *CallbackEvent) {
	if e := recover(); e != nil {
		s.events = CAN_END
		s.timeout = 0
		config.error = ERR_CALLBACK_PANIC
	}
}

func (s *CallBackJob) SetEvents(events uint32) error {
	s.Lock.Lock()
	defer s.Lock.Unlock()
	s.events = events
	s.timeout = 0
	if s.worker != nil {
		return s.worker.pushJobConfig(s.jobId)
	}
	return nil
}
func (s *CallBackJob) Release() error {
	s.Lock.Lock()
	defer s.Lock.Unlock()
	s.events = CAN_END
	s.timeout = 0
	if s.worker != nil {
		return s.worker.pushJobConfig(s.jobId)
	}
	return nil
}

func (s *CallBackJob) GetSettings() (fd int32, events uint32, timeout int64, cb func(*CallbackEvent)) {
	s.Lock.RLock()
	defer s.Lock.RUnlock()
	fd = s.fd
	events = s.events
	timeout = s.timeout
	cb = s.onEvent
	return
}

func (s *CallBackJob) SetCallback(cb func(*CallbackEvent)) {
	s.Lock.Lock()
	defer s.Lock.Unlock()
	s.onEvent = cb
}

func (s *CallBackJob) JobId() int64 {
	return s.jobId
}

func (s *CallBackJob) Fd() int32 {
	return s.fd
}

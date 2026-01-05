package osfp

import (
	"errors"

	"golang.org/x/sys/unix"
)

// Used to shutdown a new Job, when no events were aded
var ERR_NO_EVENTS = errors.New("No watchEvents returned")

func (s *ControlJob) computeFutureT(f, t int64) (futureTimeOut int64) {
	if f > 0 {

		if f > t {
			futureTimeOut = t
		}
	} else {

		futureTimeOut = t
	}
	return
}
func (s *ControlJob) ProcessEvents(currentEvents int16, now int64) (watchEevents int16, futureTimeOut int64, EventError error) {
	worker := s.worker
	if currentEvents&IN_ERROR != 0 {
		watchEevents = 0
		futureTimeOut = 0
		EventError = ERR_SHUTDOWN
		worker.write.Close()
		worker.closed = true
		return
	}

	_, EventError = worker.read.Read(s.buffer)
	if EventError != nil {
		EventError = ERR_SHUTDOWN
		worker.write.Close()
		worker.closed = true
		return
	}
	// only set watch events, once we pass our error checks
	watchEevents = CAN_READ
	// in the worker our next is current and our current is next
	// so we need to convert state in orer
	state := worker.state
	fds := worker.fds[state]
	jobs := worker.jobs[state]
	jobst := worker.jobst[state]

	que := s.worker.que
CTRL_LOOP:
	for usage, limit := s.worker.LocalPoolUsage(); usage <= limit; {
		select {
		case job := <-que:
			events, t, fd := job.SetPool(worker, now)
			if events == 0 {
				if t <= 0 {
					job.ClearPool(ERR_NO_EVENTS)
					continue
				}
				futureTimeOut = s.computeFutureT(futureTimeOut, t)
				*jobst = append(*jobst, &JobContainer{Job: job, nextTs: t})
			} else {
				futureTimeOut = s.computeFutureT(futureTimeOut, t)
				p := unix.PollFd{
					Fd:     int32(fd),
					Events: events,
				}
				*fds = append(*fds, p)
				*jobs = append(*jobs, &JobContainer{Job: job, nextTs: t})
			}
		default:
			// No more jobs to add
			break CTRL_LOOP
		}
	}

	return
}

// Called to validate the "lastTimeout", should return a futureTimeOut or 0 if there is no timeout.
// If the Job has timed out TimeOutError should be set to os.ErrDeadlineExceeded.
func (s *ControlJob) CheckTimeOut(now int64, lastTimeout int64) (NewEvents int16, futureTimeOut int64, TimeOutError error) {
	return 0, 0, nil
}

// Implemented only for interface compliance.
func (s *ControlJob) SetPool(worker *Worker, now int64) (watchEevents int16, futureTimeOut int64, fd int32) {
	s.worker = worker
	s.buffer = make([]byte, 50)
	watchEevents = CAN_READ
	futureTimeOut = 0
	fd = int32(worker.read.Fd())
	for i := range 2 {
		fds := (worker.fds[i])
		jobs := (worker.jobs[i])

		*fds = append(*fds, unix.PollFd{Events: watchEevents, Fd: fd})
		*jobs = append(*jobs, &JobContainer{Job: s})
	}
	return
}

// Implemented only for interface compliance.
func (s *ControlJob) ClearPool(_ error) {
	s.worker = nil
	s.buffer = nil
}

type ControlJob struct {
	worker *Worker
	buffer []byte
}

func NewControlJob() *ControlJob {
	return &ControlJob{
		buffer: make([]byte, 50),
	}
}

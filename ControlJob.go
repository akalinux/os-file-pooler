package osfp

import (
	"errors"

	"golang.org/x/sys/unix"
)

// Used to shutdown a new Job, when no events were aded
var ERR_NO_EVENTS = errors.New("No watchEvents returned")

func (s *ControlJob) ProcessEvents(currentEvents int16, now int64) (watchEevents int16, futureTimeOut int64, EventError error) {
	worker := s.worker
	if currentEvents&IN_ERROR != 0 {
		watchEevents = 0
		futureTimeOut = 0
		EventError = ERR_SHUTDOWN
		worker.write.Close()
		return
	}
	watchEevents = CAN_READ

	que := s.worker.que
	_, EventError = s.worker.read.Read(s.buffer)
	if EventError != nil {
		EventError = errors.Join(ERR_SHUTDOWN, EventError)
		worker.write.Close()
	}
	// in the worker our next is current and our current is next
	// so we need to convert state in orer
	state := (s.worker.state + 1) & 1
	fds := worker.fds[state]
	jobs := worker.jobs[state]

	for worker.limit < len(worker.jobs)+len(*jobs)-1 {
		select {
		case job := <-que:
			events, t, fd := job.SetPool(worker, now)
			if events == 0 {
				job.ClearPool(ERR_NO_EVENTS)
				continue
			}
			if t > 0 && futureTimeOut > t {
				futureTimeOut = t
			}
			p := unix.PollFd{
				Fd:     int32(fd),
				Events: events,
			}
			*fds = append(*fds, p)
			*jobs = append(*jobs, &JobContainer{Job: job, nextTs: t})
		default:
			// No more jobs to add
			return
		}
	}

	return
}

// Called to validate the "lastTimeout", should return a futureTimeOut or 0 if there is no timeout.
// If the Job has timed out TimeOutError should be set to os.ErrDeadlineExceeded.
func (s *ControlJob) CheckTimeOut(now int64, lastTimeout int64) (futureTimeOut int64, TimeOutError error) {
	return 0, nil
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
		if len(*jobs) != 0 || len(*fds) != 0 {
			panic("Invalid worker state, there can be only one control job, and it it must be the frist job")
		}
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

package osfp

import (
	"errors"

	"golang.org/x/sys/unix"
)

// Used to shutdown a new Job, when no events were aded
var ERR_NO_EVENTS = errors.New("No watchEvents returned")

func (s *controlJob) ProcessEvents(currentEvents uint32, now int64) (watchEevents uint32, futureTimeOut int64, EventError error) {
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

	que := s.worker.que
CTRL_LOOP:
	for {
		if worker.limit != 0 {
			usage, limit := s.worker.LocalPoolUsage()
			if usage >= limit {
				break CTRL_LOOP
			}
		}
		select {
		case job := <-que:
			events, t, fd := job.SetPool(s.worker, now)
			if events == 0 {
				if t <= 0 {
					job.ClearPool(ERR_NO_EVENTS)
					continue CTRL_LOOP
				}
			} else {
				c := &wjc{
					Job:    job,
					nextTs: t,
					wanted: events,
				}
				worker.fdjobs[fd] = c
				worker.jobs[job.JobId()] = c
			}

			if t != 0 {

				if m, ok := worker.timeouts.Get(t); ok {
					m[job.JobId()] = job
				} else {
					worker.timeouts.Put(t, map[int64]Job{t: job})
				}
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
func (s *controlJob) CheckTimeOut(now int64, lastTimeout int64) (NewEvents uint32, futureTimeOut int64, TimeOutError error) {
	return 0, 0, nil
}

// Implemented only for interface compliance.
func (s *controlJob) SetPool(worker *Worker, now int64) (watchEevents uint32, futureTimeOut int64, fd int32) {
	s.worker = worker
	s.buffer = make([]byte, max(s.worker.limit, UNLIMITED_QUE_SIZE))
	watchEevents = CAN_READ
	futureTimeOut = 0
	fd = int32(worker.read.Fd())

	return
}

// Implemented only for interface compliance.
func (s *controlJob) ClearPool(_ error) {
	unix.Close(s.worker.epfd)
	s.worker = nil
	s.buffer = nil
}

type controlJob struct {
	worker *Worker
	buffer []byte
	jobId  int64
	fd     int32
}

func (s *controlJob) Fd() int32 {
	return s.fd
}

func newControlJob() *controlJob {
	return &controlJob{
		buffer: make([]byte, 0xff),
		jobId:  nextJobId(),
	}
}

func (s *controlJob) Release() error {
	return nil
}

func (s *controlJob) JobId() int64 {
	return s.jobId
}

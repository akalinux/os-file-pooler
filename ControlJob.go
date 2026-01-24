package osfp

import (
	"errors"

	"golang.org/x/sys/unix"
)

// Used to shutdown a new Job, when no events were aded
var ERR_NO_EVENTS = errors.New("No watchEvents returned")

// Linux pipe buffer size is 65536.
var WORKER_BUFFER_SIZE = 0xffff

func (s *controlJob) ProcessEvents(currentEvents uint32, now int64) (watchEevents uint32, futureTimeOut int64, EventError error) {
	worker := s.worker
	if worker == nil {
		EventError = ERR_SHUTDOWN
		return
	}

	if currentEvents&IN_ERROR != 0 {
		watchEevents = 0
		futureTimeOut = 0
		EventError = ERR_SHUTDOWN
		worker.write.Close()
		worker.closed = true
		return
	}

	watchEevents = CAN_READ
	configs, EventError := worker.getChanges()
	defer clear(configs)
	if EventError != nil {
		EventError = ERR_SHUTDOWN
		worker.write.Close()
		worker.closed = true
		return
	}
	for jobid := range configs {
		if job, ok := worker.jobs[jobid]; ok {
			events, t, fd := job.SetPool(worker, now, jobid)

			if fd != -1 {
				// events management
				if events != job.wanted {
					// need to remove from watchers no mater what
					// changed
					if events == 0 {
						// need to remove from our watcehrs
						worker.clearJob(job, nil)
						return
					}
				}
				worker.changeEvents(job, events)
				worker.resetTimeout(job, t)

			} else {
				worker.resetTimeout(job, t)
			}
		}
	}
	// release those configs baby!

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
			s.AddJob(job, now)
		default:
			// No more jobs to add
			break CTRL_LOOP
		}
	}

	return
}

func (s *controlJob) AddJob(job Job, now int64) {
	worker := s.worker

	events, t, fd := job.SetPool(worker, now, worker.nextJobId())
	c := &wjc{
		Job:    job,
		nextTs: t,
		wanted: events,
	}
	if events == 0 {
		if t <= 0 {
			job.ClearPool(ERR_NO_EVENTS)
			return
		}
		s.addTimeoutJob(job, t)
	} else {
		e := unix.EpollCtl(worker.epfd, unix.EPOLL_CTL_ADD, int(fd), &unix.EpollEvent{Events: events, Fd: fd})
		if e != nil {
			job.ClearPool(e)
			return
		}
		worker.fdjobs[fd] = c
	}

	if t > 0 {
		s.addTimeoutJob(job, t)
	}

	worker.jobs[job.JobId()] = c
}

func (s *controlJob) addTimeoutJob(job Job, t int64) {
	if m, ok := s.worker.timeouts.Get(t); ok {
		m[job.JobId()] = job
	} else {
		s.worker.timeouts.Put(t, map[int64]Job{job.JobId(): job})
	}
}

// Called to validate the "lastTimeout", should return a futureTimeOut or 0 if there is no timeout.
// If the Job has timed out TimeOutError should be set to os.ErrDeadlineExceeded.
func (s *controlJob) CheckTimeOut(now int64, lastTimeout int64) (NewEvents uint32, futureTimeOut int64, TimeOutError error) {
	return CAN_READ, 0, nil
}

// Implemented only for interface compliance.
func (s *controlJob) SetPool(worker *Worker, now int64, jobId int64) (watchEevents uint32, futureTimeOut int64, fd int32) {
	s.worker = worker
	s.jobId = jobId
	s.buffer = make([]byte, WORKER_BUFFER_SIZE)
	watchEevents = CAN_READ
	futureTimeOut = 0
	fd = int32(worker.read.Fd())

	return
}

// Implemented only for interface compliance.
func (s *controlJob) ClearPool(_ error) {
	if s.worker != nil {
		s.worker.timeouts.RemoveAll()
		s.worker.closed = true
		for _, job := range s.worker.jobs {
			if job.Fd() > -1 {
				unix.EpollCtl(s.worker.epfd, unix.EPOLL_CTL_DEL, int(job.Fd()), nil)
			}
			job.InEventLoop()
			job.ClearPool(ERR_SHUTDOWN)
		}
		unix.Close(s.worker.epfd)
		s.worker.fdjobs = nil
	}
	s.worker = nil
	s.buffer = nil
}

type controlJob struct {
	worker  *Worker
	buffer  []byte
	jobId   int64
	fd      int32
	backlog []byte
}

func (s *controlJob) Fd() int32 {
	return s.fd
}

func newControlJob() *controlJob {
	return &controlJob{
		buffer: make([]byte, WORKER_BUFFER_SIZE),
		// Always set the control job to -3
		jobId:   -3,
		backlog: make([]byte, 0, INT64_SIZE),
	}
}

func (s *controlJob) Release() error {
	return nil
}

func (s *controlJob) JobId() int64 {
	return s.jobId
}

func (s *controlJob) InEventLoop() {
	// stub, required for the interface
}

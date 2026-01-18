package osfp

import (
	"errors"
	"fmt"

	"golang.org/x/sys/unix"
)

// Used to shutdown a new Job, when no events were aded
var ERR_NO_EVENTS = errors.New("No watchEvents returned")

func (s *controlJob) ProcessEvents(currentEvents uint32, now int64) (watchEevents uint32, futureTimeOut int64, EventError error) {
	if s.worker == nil {
		EventError = ERR_SHUTDOWN
		return
	}
	worker := s.worker
	if currentEvents&IN_ERROR != 0 {
		fmt.Printf("got error in control job\n")
		watchEevents = 0
		futureTimeOut = 0
		EventError = ERR_SHUTDOWN
		worker.write.Close()
		worker.closed = true
		return
	}

	_, EventError = worker.read.Read(s.buffer)
	if EventError != nil {
		fmt.Printf("got read error in control job\n")
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
			fmt.Printf("Added job: \n")
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
	events, t, fd := job.SetPool(s.worker, now)
	c := &wjc{
		Job:    job,
		nextTs: t,
		wanted: events,
	}
	fmt.Printf("Got job id: %d\n", job.JobId())
	if events == 0 {
		if t <= 0 {
			fmt.Printf("Job has nothing for us to do\n")
			job.ClearPool(ERR_NO_EVENTS)
			return
		}
		s.addTimeoutJob(job, t)
	} else {
		fmt.Printf("Adding job\n")
		e := unix.EpollCtl(worker.epfd, unix.EPOLL_CTL_ADD, int(fd), &unix.EpollEvent{Events: events, Fd: fd})
		if e != nil {
			job.ClearPool(e)
			fmt.Printf("Giving up because we could not add fd to poller, error was: %v\n", e)
			return
		}
		worker.fdjobs[fd] = c
	}

	if t > 0 {
		fmt.Printf("Adding Timout for job\n")
		s.addTimeoutJob(job, t)
	}
	worker.jobs[job.JobId()] = c
}

func (s *controlJob) addTimeoutJob(job Job, t int64) {
	if m, ok := s.worker.timeouts.Get(t); ok {
		m[job.JobId()] = job
	} else {
		s.worker.timeouts.Put(t, map[int64]Job{t: job})
	}
}

// Called to validate the "lastTimeout", should return a futureTimeOut or 0 if there is no timeout.
// If the Job has timed out TimeOutError should be set to os.ErrDeadlineExceeded.
func (s *controlJob) CheckTimeOut(now int64, lastTimeout int64) (NewEvents uint32, futureTimeOut int64, TimeOutError error) {
	return CAN_READ, 0, nil
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
	if s.worker != nil {
		unix.Close(s.worker.epfd)
		s.worker.timeouts.RemoveAll()
		s.worker.fdjobs = nil
		fmt.Printf("Got here, need to close our owner\n")
		s.worker.closed = true
		for _, job := range s.worker.jobs {
			job.ClearPool(ERR_SHUTDOWN)
		}
	}
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

package osfp

import (
	"errors"

	"golang.org/x/sys/unix"
)

// Used to shutdown a new Job, when no events were aded
var ERR_NO_EVENTS = errors.New("No watchEvents returned")

// Lnix pipe buffer size is 65536, since we need a multiple of 8 we subtract 7
var WORKER_BUFFER_SIZE = 0xffff - 7

func (s *controlJob) ProcessEvents(currentEvents uint32, now int64) (watchEevents uint32, futureTimeOut int64, EventError error) {
	if s.worker == nil {
		EventError = ERR_SHUTDOWN
		return
	}
	futureTimeOut = -1
	worker := s.worker
	if currentEvents&IN_ERROR != 0 {
		watchEevents = 0
		futureTimeOut = 0
		EventError = ERR_SHUTDOWN
		worker.write.Close()
		worker.closed = true
		return
	}

	size, EventError := worker.read.Read(s.buffer)
	if EventError != nil {
		EventError = ERR_SHUTDOWN
		worker.write.Close()
		worker.closed = true
		return
	}

	watchEevents = CAN_READ
	if ok := s.processBuffer(size, now); !ok {
		return
	}

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

func (s *controlJob) processBuffer(size int, now int64) (run bool) {
	if s.worker == nil {
		return
	}
	worker := s.worker
	s.byteReader(size, func(jobid int64) {
		if jobid == WAKEUP_THREAD {
			run = true
		} else if job, ok := worker.jobs[jobid]; ok {
			events, t, fd := job.SetPool(worker, now)

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
	})
	return
}

// broken out to make this very unit testable!
func (s *controlJob) byteReader(size int, cb func(int64)) {
	if size == 0 {
		return
	}

	// get first chunk
	begin := len(s.backlog)
	end := INT64_SIZE - begin
	if size+begin < INT64_SIZE {
		// if we get here the buffer is too small and we just need to save the backlog and return
		s.backlog = append(s.backlog, s.buffer[0:size]...)
		return
	} else if begin != 0 {
		// fill our backlog to our end
		s.backlog = append(s.backlog, s.buffer[0:end]...)
		cb(bytesToInt64(s.backlog))
		s.backlog = s.backlog[:0]
		begin = end + 1
	}

	for {
		end = begin + INT64_SIZE
		if size >= end {
			// got a job id of some kind!
			cb(bytesToInt64(s.buffer[begin:end]))
		} else if begin < size {
			// save our backlog
			s.backlog = s.buffer[begin:size]
			return
		} else {
			// empty our backlog
			s.backlog = s.backlog[:0]
			return
		}

		begin += INT64_SIZE
	}
}

func (s *controlJob) AddJob(job Job, now int64) {
	worker := s.worker

	events, t, fd := job.SetPool(s.worker, now)
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
func (s *controlJob) SetPool(worker *Worker, now int64) (watchEevents uint32, futureTimeOut int64, fd int32) {
	s.worker = worker
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
		for id, job := range s.worker.jobs {
			if id == s.jobId {
				continue
			}
			job.ClearPool(ERR_SHUTDOWN)
		}
		unix.Close(s.worker.epfd)
	}
	s.worker.fdjobs = nil
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
		buffer:  make([]byte, WORKER_BUFFER_SIZE),
		jobId:   nextJobId(),
		backlog: make([]byte, 0, INT64_SIZE),
	}
}

func (s *controlJob) Release() error {
	return nil
}

func (s *controlJob) JobId() int64 {
	return s.jobId
}

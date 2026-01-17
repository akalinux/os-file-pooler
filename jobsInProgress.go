package osfp

import (
	omap "github.com/akalinux/orderedmap"
	"golang.org/x/sys/unix"
)

const POLLIN = unix.POLLIN
const POLLOUT = unix.POLLOUT

// check for end of can read and write
const RW_COMPLETE = unix.EAGAIN

// Need to upgrade to epoll7:
// https://man7.org/linux/man-pages/man7/epoll.7.html
// https://pkg.go.dev/golang.org/x/sys/unix#EpollCreate

type jobsInProgress struct {
	timeouts     omap.SliceTree[int64, map[int64]Job]
	fds          map[int32]*wjc
	worker       *Worker
	now          int64
	pollInCount  int
	pollOutCount int
}

func (s *jobsInProgress) UpdateEvents(e int16, job Job) {}
func (s *jobsInProgress) AddJob(job Job) {
	events, futureTimeOut, fd := job.SetPool(s.worker, s.now)
	if events == 0 {
		if futureTimeOut <= 0 {
			job.ClearPool(ERR_NO_EVENTS)
			s.ClearJob(job)
			return
		}
	} else {
		s.worker.addPoll(&unix.PollFd{Fd: fd, Events: events})
		s.fds[fd] = &wjc{futureTimeOut, job}
	}

	if futureTimeOut != 0 {

		if m, ok := s.timeouts.Get(futureTimeOut); ok {
			m[job.JobId()] = job
		} else {
			s.timeouts.Put(futureTimeOut, map[int64]Job{futureTimeOut: job})
		}
	}
}

func (s *jobsInProgress) ClearJob(job Job) {

}
func (s *jobsInProgress) RunTimeouts(now int64) {
	for to, jobs := range s.timeouts.RemoveBetweenKV(-1, now, omap.FIRST_KEY) {
		for _, job := range jobs {
			w, ft, e := job.CheckTimeOut(now, to)
			if e != nil {
				job.ClearPool(e)
				s.ClearJob(job)
			}
			if w != 0 {
				s.UpdateEvents(w, job)
			}
			if ft > 0 {
				if m, ok := s.timeouts.Get(ft); ok {
					m[job.JobId()] = job
				} else {
					s.timeouts.Put(ft, map[int64]Job{ft: job})
				}
			}
		}
	}
}

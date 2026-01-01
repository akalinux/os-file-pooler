package osfp

import (
	"errors"
	"os"
	"sync"
	"time"

	"golang.org/x/sys/unix"
)

var ERR_SHUTDOWN = errors.New("Thread pool Shutdown")

// https://man7.org/linux/man-pages/man2/poll.2.html
const IN_ERROR = unix.POLLERR | // Errors
	unix.POLLHUP | // Other end has disconnected
	unix.POLLNVAL // Other end has closed
// Checks if an fd can read
// Take a look t the unix poller constants for more info: unix.POLLER*.
const CAN_READ = unix.POLLIN

// checks if an fd can write
// Take a look t the unix poller constants for more info: unix.POLLER*.
const CAN_WRITE = unix.POLLOUT

type Worker struct {
	throttle chan any
	que      chan Job
	file     *os.File
	limit    int
	state    byte
	jobs     [][]Job
	fds      [][]unix.PollFd
	nextTs   int64
	closed   bool
	locker   sync.Mutex
}

func NewWorker(que chan Job, throttle chan any, file *os.File, limit int) *Worker {
	return &Worker{
		throttle: throttle,
		que:      que,
		file:     file,
		limit:    limit,
		nextTs:   -1,
		closed:   false,
	}
}

func (s *Worker) Wakeup() {
	s.locker.Lock()
	defer s.locker.Unlock()
	unix.Write(int(s.file.Fd()), []byte{0})
}

func (s *Worker) Run() {

	for !s.closed {
		s.SingleRun()
	}
}

func (s *Worker) SingleRun() {
	now := time.Now().UnixMilli()
	var sleep int64 = -1
	if s.nextTs > 0 {
		diff := s.nextTs - now
		sleep = max(diff, 0)
	}

	// current slices id
	currentState := s.state
	// alternate 0 1 and back again
	s.state = (s.state + 1) & 1
	nextState := s.state
	currentFd := s.fds[currentState]
	nextFd := s.fds[nextState]
	// reset our next
	nextFd = nextFd[:1]

	// swap current and next
	s.fds[nextState] = currentFd
	s.fds[currentState] = nextFd

	currentJobs := s.jobs[currentState]
	nextJobs := s.jobs[nextState]
	// reset next jobs to defaults
	nextJobs = nextJobs[:1]
	s.jobs[currentState] = nextJobs
	s.jobs[nextState] = currentJobs

	active, err := unix.Poll(currentFd, int(sleep))
	if err != nil {
		s.closed = true
		for _, job := range currentJobs {
			job.Shutdown(err)
			job.ClearPool()
		}
		return
	}
	s.WalkJobs(currentState, nextState, active, now, sleep)

}

func (s *Worker) WalkJobs(cs, ns byte, active int, now, sleep int64) {
	s.nextTs = -1

	// first job and file fd are always our control job.
	job := s.jobs[cs][0]
	fd := s.fds[cs][0]
	var err error
	var nextTs int64 = -1
	if fd.Revents != 0 {
		// Process our control job first
		flags, sleep := job.ProcessFlags(fd.Revents, now)
		nextTs = sleep
		if flags == 0 {
			// if we get here the fd is closed!
			err = errors.New("Shutdown in progress!")
		}
	}
	if err != nil {
		s.closed = true
		for i := 1; i < len(s.jobs[cs]); i++ {
			s.jobs[cs][i].Shutdown(err)
			s.jobs[cs][i].ClearPool()
		}
	} else {
		s.processNextSet(cs, ns, now, nextTs)
	}

}

func (s *Worker) processNextSet(cs, ns byte, now int64, StarterNextTs int64) {
	var nextTs int64 = StarterNextTs
	for i := 1; i < len(s.jobs[cs]); i++ {
		fd := s.fds[cs][i]
		job := s.jobs[cs][i]
		events := fd.Events
		fd.Revents = 0
		flags, futureTs := job.ProcessFlags(events, now)
		fd.Events = flags
		if flags == 0 {
			job.ClearPool()
			// remove one job from the pool overall
			<-s.throttle
			continue
		}
		s.fds[ns] = append(s.fds[ns], fd)
		s.jobs[ns] = append(s.jobs[ns], job)
		if futureTs > 0 {
			if nextTs > 0 {
				if nextTs > futureTs {
					nextTs = futureTs
				}
			} else {
				nextTs = futureTs
			}
		}

	}
	s.nextTs = nextTs
}

func (s *Worker) Close() {
	if s.closed {
		return
	}
	s.locker.Lock()
	defer s.locker.Unlock()
	s.file.Close()
}

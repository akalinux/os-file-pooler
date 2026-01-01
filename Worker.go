package osfp

import (
	"errors"
	"io"
	"os"
	"sync"
	"time"

	"golang.org/x/sys/unix"
)

const EPOLL_RETRY_TIMEOUT = 10

var ERR_SHUTDOWN = errors.New("Thread pool Shutdown")

// https://man7.org/linux/man-pages/man2/poll.2.html

const (
	IN_ERROR = unix.POLLERR | // Errors
		unix.POLLHUP | // Other end has disconnected
		unix.POLLNVAL // Not a valid handle

	// catch all for EOF
	IN_EOF = unix.POLLHUP | unix.POLLRDHUP

	// Checks if an fd can read
	// Take a look t the unix poller constants for more info: unix.POLLER*.
	CAN_READ = unix.POLLIN

	// checks if an fd can write
	// Take a look t the unix poller constants for more info: unix.POLLER*.
	CAN_WRITE = unix.POLLOUT
)

type Worker struct {
	throttle chan any
	que      chan Job
	file     *os.File
	limit    int
	state    byte
	jobs     [][]*JobContainer
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
		currentState, nextState, now, sleep := s.NextState()
		active, e := s.doPoll(currentState, sleep)
		if e != nil {
			continue
		}
		s.walkJobs(currentState, nextState, now, active)
	}
}

func (s *Worker) NextState() (currentState byte, nextState byte, now int64, sleep int64) {
	now = time.Now().UnixMilli()
	sleep = -1
	if s.nextTs > 0 {
		diff := s.nextTs - now
		sleep = max(diff, 0)
	}

	// current slices id
	currentState = s.state
	// alternate 0 1 and back again
	s.state = (s.state + 1) & 1
	nextState = s.state
	currentFd := s.fds[currentState]
	nextFd := s.fds[nextState]

	// swap current and next
	s.fds[nextState] = currentFd
	s.fds[currentState] = nextFd

	currentJobs := s.jobs[currentState]
	nextJobs := s.jobs[nextState]
	s.jobs[currentState] = nextJobs
	s.jobs[nextState] = currentJobs

	// reset the next objects to the defaults
	nextJobs = nextJobs[:1]
	nextFd = nextFd[:1]
	return
}

func (s *Worker) doPoll(cs byte, sleep int64) (active int, err error) {
	active, err = unix.Poll(s.fds[cs], int(sleep))
	return
}

func (s *Worker) walkJobs(cs, ns byte, now int64, active int) {

	s.nextTs = -1

	// first job and file fd are always our control job.
	job := s.jobs[cs][0]
	fd := s.fds[cs][0]
	var nextTs int64 = -1
	var err error
	if active != 0 && fd.Revents != 0 {
		// Process our control job first
		flags, sleep := job.ProcessEvents(fd.Revents, now)
		nextTs = sleep
		if flags == 0 {
			// if we get here the fd is closed!
			err = ERR_SHUTDOWN
		}
	}
	if err != nil {
		s.closed = true
		for i := 1; i < len(s.jobs[cs]); i++ {
			s.clearJob(s.jobs[cs][i], err)
		}
	} else {
		s.processNextSet(cs, ns, now, nextTs)
	}

}
func (s *Worker) clearJob(job Job, e error) {
	job.ClearPool(e)
	<-s.throttle
}

func (s *Worker) processNextSet(cs, ns byte, now int64, StarterNextTs int64) {
	var nextTs int64 = StarterNextTs
	for i := 1; i < len(s.jobs[cs]); i++ {
		fd := s.fds[cs][i]
		job := s.jobs[cs][i]
		events := fd.Events
		check := events & fd.Events
		var flags int16
		var futureTs int64
		if check == 0 {
			// start our error check states
			if job.nextTs != 0 {
				if job.nextTs <= now {
					// Job has timed out
					s.clearJob(job, os.ErrDeadlineExceeded)
					continue
				}
			}
			// if we get here, we need to check for errors dirrectly
			switch {
			case events&IN_EOF != 0:
				// best guess is EOF.. may be a few others
				// like io.ErrClosedPipe
				s.clearJob(job, io.EOF)
				continue
			case events&IN_ERROR != 0:
				s.clearJob(job, io.ErrUnexpectedEOF)
				continue
			}

		} else {
			flags, futureTs = job.ProcessEvents(check, now)
		}
		if flags == 0 {
			// happy ending!
			s.clearJob(job, nil)
			continue
		}
		fd.Events = flags
		fd.Revents = 0
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
	s.locker.Lock()
	defer s.locker.Unlock()
	if s.closed {
		return
	}
	s.file.Close()
}

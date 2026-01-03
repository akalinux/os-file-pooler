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
var ERR_QUE_FULL = errors.New("Queue is full")

// https://man7.org/linux/man-pages/man2/poll.2.html
const (
	IN_ERROR = int16(unix.POLLERR | // Errors
		unix.POLLHUP | // Other end has disconnected
		unix.POLLNVAL) // Not a valid handle

	// catch all for EOF
	IN_EOF = int16(unix.POLLHUP | unix.POLLRDHUP)

	// Checks if an fd can read
	// Take a look t the unix poller constants for more info: unix.POLLER*.
	CAN_READ = int16(unix.POLLIN)

	// checks if an fd can write
	// Take a look t the unix poller constants for more info: unix.POLLER*.
	CAN_WRITE = int16(unix.POLLOUT)

	// watch both read and write events
	CAN_RW = int16(CAN_WRITE | CAN_READ)

	// Tell the Pool to release this element
	CAN_END = int16(0)
)

type Worker struct {
	throttle chan any
	que      chan Job
	read     *os.File
	write    *os.File
	limit    int
	state    byte
	jobs     []*[]*JobContainer
	fds      []*[]unix.PollFd
	nextTs   int64
	closed   bool
	locker   sync.Mutex
}

func NewLocalWorker(limit int) (worker *Worker, osErr error) {
	r, w, e := os.Pipe()
	osErr = e
	if e != nil {
		return nil, e
	}
	worker = NewWorker(
		make(chan Job, 1),
		make(chan any, limit),
		r, w,
		limit,
	)
	return
}
func NewStandAloneWorker() (worker *Worker, osErr error) {

	return NewLocalWorker(DEFAULT_WORKER_HANDLES)
}
func NewWorker(que chan Job, throttle chan any, read *os.File, write *os.File, limit int) *Worker {
	this := &Worker{
		throttle: throttle,
		que:      que,
		limit:    limit,
		nextTs:   -1,
		closed:   false,
		read:     read,
		write:    write,
		jobs: []*[]*JobContainer{
			{},
			{},
		},
		fds: []*[]unix.PollFd{
			{},
			{},
		},
	}
	cg := NewControlJob()
	cg.SetPool(this, time.Now().UnixMilli())
	return this
}

func (s *Worker) JobCount() (JobCount int) {
	JobCount = len(*s.jobs[s.state]) - 1
	return
}

func (s *Worker) Wakeup() (err error) {
	s.locker.Lock()
	defer s.locker.Unlock()
	if s.closed {
		return ERR_SHUTDOWN
	}
	_, err = s.write.Write([]byte{0})
	if err != nil {
		s.write.Close()
	}
	return
}

func (s *Worker) Run() {

	for !s.closed {
		currentState, nextState, now, sleep := s.NextState()
		active, e := s.DoPoll(currentState, sleep)
		if e != nil {
			continue
		}
		s.WalkJobs(currentState, nextState, now, active)
	}
}

func (s *Worker) SingleRun() error {
	currentState, nextState, now, sleep := s.NextState()

	active, e := s.DoPoll(currentState, sleep)
	if e != nil {
		return e
	}
	s.WalkJobs(currentState, nextState, now, active)

	return nil
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

	nextFd := *s.fds[nextState]
	nextJobs := *s.jobs[nextState]

	nextFd = nextFd[:1]
	nextJobs = nextJobs[:1]
	s.fds[nextState] = &nextFd
	s.jobs[nextState] = &nextJobs

	return
}

func (s *Worker) DoPoll(cs byte, sleep int64) (active int, err error) {
	active, err = unix.Poll(*s.fds[cs], int(sleep))
	return
}

func (s *Worker) WalkJobs(cs, ns byte, now int64, active int) {

	s.nextTs = -1

	// first job and file fd are always our control job.
	job := (*s.jobs[cs])[0]
	fd := (*s.fds[cs])[0]
	var nextTs int64 = -1
	var err error
	if active != 0 && fd.Revents != 0 {
		// Process our control job first
		flags, sleep, _ := job.ProcessEvents(fd.Revents, now)
		nextTs = sleep
		if flags == 0 {
			// if we get here the fd is closed!
			err = ERR_SHUTDOWN
		}
	}
	if err != nil {
		s.closed = true
		for i := 1; i < len(*s.jobs[cs]); i++ {
			s.clearJob((*s.jobs[cs])[i], err)
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
	currentFds := (s.fds[cs])
	nextFds := (s.fds[ns])
	currentJobs := (s.jobs[cs])
	nextJobs := (s.jobs[ns])

	loopSize := len(*currentJobs)
	for i := 1; i < loopSize; i++ {
		fd := (*currentFds)[i]
		job := (*currentJobs)[i]
		events := fd.Revents
		check := events & fd.Events
		var flags int16
		var futureTs int64
		var err error
		if check == 0 {
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
			// start our error check states
			if newTs, err := job.CheckTimeOut(now, job.nextTs); err != nil {
				s.clearJob(job, err)
				continue
			} else {
				job.nextTs = newTs
				flags = fd.Events
			}

		} else {
			flags, futureTs, err = job.ProcessEvents(check, now)
		}
		if flags == 0 {
			// happy ending!
			s.clearJob(job, err)
			continue
		}
		fd.Events = flags
		fd.Revents = 0
		*nextFds = append(*nextFds, fd)
		*nextJobs = append(*nextJobs, job)
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

func (s *Worker) Close() error {
	s.locker.Lock()
	defer s.locker.Unlock()
	if s.closed {
		// safe to call on a closed handle
		s.write.Close()
		return ERR_SHUTDOWN
	}
	// Safe to call lock multiple times.. or even on multiple threads.
	// Just pass the error back if it is an issue.
	return s.write.Close()
}

func (s *Worker) AddJob(job Job) (err error) {
	if s.closed {
		err = ERR_SHUTDOWN
	} else {
		s.throttle <- struct{}{}
		s.que <- job
		err = s.Wakeup()
	}

	return
}
func (s *Worker) TryAddJob(job Job) (err error) {
	if s.closed {
		err = ERR_SHUTDOWN
	} else {
		select {

		case s.throttle <- struct{}{}:
			s.que <- job
			err = s.Wakeup()
		default:
			err = ERR_QUE_FULL
		}
	}

	return
}

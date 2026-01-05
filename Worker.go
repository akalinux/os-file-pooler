package osfp

import (
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"golang.org/x/sys/unix"
)

const EPOLL_RETRY_TIMEOUT = time.Millisecond * 10

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
	jobs     []*[]*wjc
	jobst    []*[]*wjc
	fds      []*[]unix.PollFd
	nextTs   int64
	closed   bool
	locker   sync.RWMutex
}

func NewLocalWorker(limit int) (worker *Worker, osErr error) {
	r, w, e := os.Pipe()
	osErr = e
	if e != nil {
		return nil, e
	}
	worker = NewWorker(
		make(chan Job, limit),
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
		jobs: []*[]*wjc{
			{},
			{},
		},
		jobst: []*[]*wjc{
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

// Returns the number of active jobs in the local pool.
func (s *Worker) JobCount() (JobCount int) {
	JobCount, _ = s.LocalPoolUsage()
	return
}

func (s *Worker) PoolUsageString() string {

	c := cap(s.throttle)
	u := len(s.throttle)
	b := len(s.que)
	x := cap(s.que)
	t, l := s.LocalPoolUsage()
	return fmt.Sprintf("Local Limits: %d/%d, Pool Capacity: %d/%d, Backlog %d/%d", t, l, u, c, b, x)
}

func (s *Worker) LocalPoolUsage() (usage, limit int) {
	s.locker.RLock()
	defer s.locker.RUnlock()
	usage = len(*s.jobs[s.state]) + len(*s.jobst[s.state]) - 1
	limit = s.limit
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
	} else {
		return err
	}
	return
}

func (s *Worker) Run() {

	for !s.closed {
		currentState, nextState, now, sleep := s.nextState()
		active, e := s.doPoll(currentState, sleep)
		if e != nil {
			// Give the os some grace time to recover
			time.Sleep(EPOLL_RETRY_TIMEOUT)
			continue
		}
		s.walkJobs(currentState, nextState, now, active)
	}
}

func (s *Worker) SingleRun() error {
	currentState, nextState, now, sleep := s.nextState()
	active, e := s.doPoll(currentState, sleep)
	if e != nil {
		return e
	}
	s.walkJobs(currentState, nextState, now, active)

	return nil
}

func (s *Worker) nextState() (currentState byte, nextState byte, now int64, sleep int64) {
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
	nextJobst := *s.jobst[nextState]

	nextFd = nextFd[:1]
	nextJobs = nextJobs[:1]
	nextJobst = nextJobst[:0]
	s.fds[nextState] = &nextFd
	s.jobs[nextState] = &nextJobs
	s.jobst[nextState] = &nextJobst

	return
}

func (s *Worker) doPoll(cs byte, sleep int64) (active int, err error) {

	if s.closed {
		err = ERR_SHUTDOWN
		return
	}
	active, err = unix.Poll(*s.fds[cs], int(sleep))
	return
}

func (s *Worker) walkJobs(cs, ns byte, now int64, active int) {

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
		for _, job := range *s.jobs[cs] {
			job.ClearPool(err)
		}
	} else {
		s.processNextSet(cs, ns, now, nextTs)
	}

}

// Call this when a job needs to be removed from the pool.
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

	fdLoopSize := len(*currentFds)
	for i := 1; i < fdLoopSize; i++ {
		fd := (*currentFds)[i]
		job := (*currentJobs)[i]
		events := fd.Revents
		check := events & fd.Events
		var flags int16
		var futureTs int64
		var err error
		if check == 0 {
			// if we get here, we need to check for errors dirrectly
			if events&IN_EOF != 0 {
				// best guess is EOF.. may be a few others
				// like io.ErrClosedPipe
				s.clearJob(job, io.EOF)
				continue
			} else if events&IN_ERROR != 0 {
				s.clearJob(job, io.ErrUnexpectedEOF)
				continue
			}
			// start our error check states

			if flags, futureTs, err = s.checkJobTs(job, now); futureTs == -1 {
				continue
			} else {
				job.nextTs = futureTs
			}

		} else {
			flags, futureTs, err = job.ProcessEvents(check, now)
		}
		if flags == 0 || err != nil {
			// callback driven ending
			s.clearJob(job, err)
			continue
		}
		fd.Events = flags
		fd.Revents = 0
		*nextFds = append(*nextFds, fd)
		*nextJobs = append(*nextJobs, job)
		nextTs = s.resolveNextTs(nextTs, futureTs)

	}

	currentJobs = (s.jobst[cs])
	nextJobs = (s.jobst[ns])
	for _, job := range *currentJobs {
		if _, futureTs, _ := s.checkJobTs(job, now); futureTs == -1 {
			continue
		} else {
			job.nextTs = futureTs
			nextTs = s.resolveNextTs(nextTs, futureTs)
		}
		*nextJobs = append(*nextJobs, job)
	}
	s.nextTs = nextTs
}

func (s *Worker) resolveNextTs(nextTs, futureTs int64) int64 {
	if futureTs > 0 {

		if nextTs > 0 {
			if nextTs > futureTs {
				return futureTs
			}
		} else {
			return futureTs
		}
	}
	return nextTs
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

func (s *Worker) checkJobTs(job *wjc, now int64) (newEvents int16, newTs int64, TimeoutError error) {
	// start our error check states
	if newEvents, newTs, err := job.CheckTimeOut(now, job.nextTs); err != nil {
		s.clearJob(job, err)
		return 0, -1, err
	} else {
		return newEvents, newTs, nil
	}
}

func (s *Worker) AddJob(job Job) (err error) {
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

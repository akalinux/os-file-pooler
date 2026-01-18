package osfp

import (
	"cmp"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	omap "github.com/akalinux/orderedmap"
	"golang.org/x/sys/unix"
)

// Need to upgrade to epoll7:
// https://man7.org/linux/man-pages/man7/epoll.7.html
// https://pkg.go.dev/golang.org/x/sys/unix#EpollCreate

const EPOLL_RETRY_TIMEOUT = time.Millisecond * 10
const UNLIMITED_QUE_SIZE = 100

var ERR_SHUTDOWN = errors.New("Thread pool Shutdown")
var ERR_QUE_FULL = errors.New("Queue is full")

// https://man7.org/linux/man-pages/man2/poll.2.html
const (
	IN_ERROR = uint32(unix.POLLERR | // Errors
		unix.POLLHUP | // Other end has disconnected
		unix.POLLNVAL) // Not a valid handle

	// catch all for EOF
	IN_EOF = uint32(unix.POLLHUP | unix.POLLRDHUP)

	// Done reading and writing
	DONE_RW = uint32(unix.EAGAIN)

	// Checks if an fd can read
	// Take a look t the unix poller constants for more info: unix.POLLER*.
	CAN_READ = uint32(unix.POLLIN)

	// checks if an fd can write
	// Take a look t the unix poller constants for more info: unix.POLLER*.
	CAN_WRITE = uint32(unix.POLLOUT)

	// watch both read and write events
	CAN_RW = uint32(CAN_WRITE | CAN_READ)

	// Tell the Pool to release this element
	CAN_END = uint32(0)
)

type Worker struct {
	throttle chan any
	que      chan Job
	read     *os.File
	write    *os.File
	limit    int
	epfd     int
	fdjobs   map[int32]*wjc
	timeouts *omap.SliceTree[int64, map[int64]Job]
	jobs     map[int64]*wjc
	nextTs   int64
	closed   bool
	locker   sync.RWMutex
	now      time.Time
	events   []unix.EpollEvent
	ctrl     *controlJob
}

func NewLocalWorker(limit int) (worker *Worker, osErr error) {
	r, w, e := os.Pipe()
	osErr = e
	if e != nil {
		return nil, e
	}
	var throttle chan any
	var que chan Job

	if limit == 0 {
		throttle = nil
		que = make(chan Job, UNLIMITED_QUE_SIZE)
	} else {
		throttle = make(chan any, limit)
		que = make(chan Job, limit)
	}
	worker, e = NewWorker(
		que,
		throttle,
		r, w,
		limit,
	)
	return
}
func NewStandAloneWorker() (worker *Worker, osErr error) {
	return NewLocalWorker(DEFAULT_WORKER_HANDLES)
}

func NewWorker(que chan Job, throttle chan any, read *os.File, write *os.File, limit int) (*Worker, error) {

	// create in level not edge mode!
	epfd, e := unix.EpollCreate1(0)
	if e != nil {
		write.Close()
		return nil, e
	}
	s := &Worker{
		throttle: throttle,
		que:      que,
		limit:    limit,
		nextTs:   -1,
		closed:   false,
		read:     read,
		write:    write,
		epfd:     epfd,
		events:   make([]unix.EpollEvent, limit+1),
		fdjobs:   make(map[int32]*wjc, limit+1),
		jobs:     make(map[int64]*wjc, limit+1),
		timeouts: omap.NewSliceTree[int64, map[int64]Job](limit, cmp.Compare),
	}

	cg := newControlJob()
	s.ctrl = cg
	_, _, jfd := cg.SetPool(s, time.Now().UnixMilli())
	e = unix.EpollCtl(epfd, unix.EPOLL_CTL_ADD, int(jfd), &unix.EpollEvent{Events: uint32(CAN_READ), Fd: jfd})
	if e != nil {
		// give up and close here
		unix.Close(epfd)
		s.closed = true
		write.Close()
		cg.ClearPool(ERR_SHUTDOWN)
		return nil, e
	}
	cj := &wjc{Job: cg, wanted: CAN_READ}

	s.jobs[cg.JobId()] = cj
	s.fdjobs[jfd] = cj

	return s, nil
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
	usage = len(s.jobs) - 1
	limit = s.limit
	return
}

func (s *Worker) Wakeup() (err error) {
	s.locker.Lock()
	defer s.locker.Unlock()
	return s.wakeup()
}

func (s *Worker) wakeup() (err error) {
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

func (s *Worker) Start() error {
	if s.closed {
		return ERR_SHUTDOWN
	}

	for !s.closed {
		s.SingleRun()
	}
	return nil
}

func (s *Worker) SingleRun(t ...int64) error {
	if len(t) != 0 {
		s.nextTs = t[0]
	}
	now, sleep := s.nextState()
	fmt.Printf("enter do poll\n")
	active, e := s.doPoll(sleep)
	if e != nil {
		return e
	}
	if active == 0 {
		return nil
	}
	fmt.Printf("Process next\n")
	s.processNextSet(now, s.nextTs, active)

	return nil
}

func (s *Worker) nextState() (now int64, sleep int64) {
	s.now = time.Now()
	now = s.now.UnixMilli()
	sleep = -1
	if s.nextTs > 0 {
		diff := s.nextTs - now

		sleep = max(diff, 0)
	}
	return
}

func (s *Worker) doPoll(sleep int64) (active int, err error) {

	if s.closed {
		err = ERR_SHUTDOWN
		return
	}
	active, err = unix.EpollWait(s.epfd, s.events, int(sleep))
	return
}

func (s *Worker) processNextSet(now int64, StarterNextTs int64, active int) {
	var nextTs int64 = StarterNextTs

	fmt.Printf("ACtive count: %d\n", active)
	for i := 0; i < active; i++ {
		events := s.events[i].Events
		fd := s.events[i].Fd

		job, ok := s.fdjobs[fd]
		if !ok {
			panic("Wooks")
		}
		fmt.Printf("Got job: %d\n", job.JobId())
		check := events & job.wanted
		if check == 0 {
			// if we get here, we need to check for errors dirrectly
			if events&IN_EOF != 0 {
				// best guess is EOF.. may be a few others
				// like io.ErrClosedPipe
				fmt.Printf("error Closing EOF\n")
				s.clearJob(job, io.EOF)
			} else if events&IN_ERROR != 0 {
				fmt.Printf("error Closing ERROR\n")
				s.clearJob(job, io.ErrUnexpectedEOF)
			}
			// start our error check states
			continue

		}
		w, t, e := job.ProcessEvents(check, now)
		if e != nil || (w == 0 && t <= 0) {
			fmt.Printf("no events ,no timeout, Closing\n")
			s.clearJob(job, e)
			continue
		}
		if lt := job.nextTs; s.updateTimeout(job, t) {
			// only clear the old job if we need to
			if t != job.nextTs {
				if m, ok := s.timeouts.Get(lt); ok {
					delete(m, job.JobId())
					if len(m) == 0 {
						s.timeouts.Remove(lt)
					}
				}
			}
		}

		s.changeEvents(job, w)
		nextTs = resolveNextTs(nextTs, t)

	}
	for lastTimeout, jobs := range s.timeouts.RemoveBetweenKV(-1, now, omap.FIRST_KEY) {
		for id, job := range jobs {
			w, t, e := job.CheckTimeOut(now, lastTimeout)
			if e != nil || (w == 0 && t <= 0) {
				s.clearJob(s.jobs[id], e)
				continue
			}
			nextTs = resolveNextTs(nextTs, t)
			s.updateTimeout(s.jobs[job.JobId()], nextTs)
		}
	}
	// if we get here then we need to process the job event.
	s.nextTs = nextTs
}

func (s *Worker) updateTimeout(job *wjc, nextTs int64) (needsCleanup bool) {
	if nextTs == job.nextTs {
		return false
	}

	needsCleanup = job.nextTs > 0
	if nextTs < 1 {
		job.nextTs = nextTs
		// stop here if we have nothing to do
		return
	}
	var m map[int64]Job
	var ok bool
	if m, ok = s.timeouts.Get(nextTs); !ok {
		m = make(map[int64]Job)
		s.timeouts.Put(nextTs, m)
	}
	job.nextTs = nextTs
	m[job.JobId()] = job
	return
}

func (s *Worker) changeEvents(job *wjc, events uint32) bool {
	fd := job.Fd()
	if job.wanted != events && fd > -1 {
		// update what we want!
		job.wanted = events
		e := unix.EpollCtl(s.epfd, unix.EPOLL_CTL_DEL, int(fd), nil)
		if e != nil {
			s.clearJob(job, e)
			return false
		}
		e = unix.EpollCtl(s.epfd, unix.EPOLL_CTL_ADD, int(fd), &unix.EpollEvent{Fd: fd, Events: events})
		if e != nil {
			s.clearJob(job, e)
			return false
		}
	}
	return true
}

// Call this when a job needs to be removed from the pool.
func (s *Worker) clearJob(job *wjc, e error) {

	fd := job.Fd()
	if fd > -1 {
		unix.EpollCtl(s.epfd, unix.EPOLL_CTL_DEL, int(fd), nil)
	}
	delete(s.fdjobs, fd)
	delete(s.jobs, job.JobId())

	if job.nextTs > 0 {
		if m, ok := s.timeouts.Get(job.nextTs); ok {
			delete(m, job.JobId())
			if len(m) == 0 {
				s.timeouts.Remove(job.nextTs)
			}
		}
	}
	job.ClearPool(e)
	if s.limit != 0 {
		<-s.throttle
	}
}

func resolveNextTs(nextTs, futureTs int64) int64 {
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

func (s *Worker) Stop() error {
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
	s.locker.RLock()
	defer s.locker.RUnlock()
	if s.closed {
		err = ERR_SHUTDOWN
		return
	}
	if s.limit == 0 {
		s.que <- job
		err = s.wakeup()
	} else {
		select {
		case s.throttle <- struct{}{}:
			s.que <- job
			err = s.wakeup()
		default:
			err = ERR_QUE_FULL
		}
	}

	return
}

func (s *Worker) NewUtil() *Util {
	return &Util{s}
}

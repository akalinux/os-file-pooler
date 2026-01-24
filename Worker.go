package osfp

import (
	"cmp"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
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

// its always 8 bytes..
const INT64_SIZE = 8

const WAKEUP_THREAD = -1

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
	closed   bool
	locker   sync.RWMutex
	now      time.Time
	events   []unix.EpollEvent
	ctrl     *controlJob
	state    byte
	pending  []map[int64]any
	buffer   []byte
	jobId    int64
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

func (s *Worker) nextJobId() int64 {
	s.jobId++
	return s.jobId
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
		closed:   false,
		read:     read,
		write:    write,
		epfd:     epfd,
		events:   make([]unix.EpollEvent, limit+1),
		fdjobs:   make(map[int32]*wjc, limit+1),
		jobs:     make(map[int64]*wjc, limit+1),
		timeouts: omap.NewSliceTree[int64, map[int64]Job](limit, cmp.Compare),
		pending: []map[int64]any{
			make(map[int64]any, UNLIMITED_QUE_SIZE),
			make(map[int64]any, UNLIMITED_QUE_SIZE),
		},
		buffer: make([]byte, WORKER_BUFFER_SIZE),
	}

	cg := newControlJob()
	s.ctrl = cg
	_, _, jfd := cg.SetPool(s, time.Now().UnixMilli(), -2)
	e = unix.EpollCtl(epfd, unix.EPOLL_CTL_ADD, int(jfd), &unix.EpollEvent{Events: CAN_READ, Fd: jfd})
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

func (s *Worker) pushJobConfig(id int64) error {
	s.locker.Lock()
	defer s.locker.Unlock()

	_, e := unix.Write(int(s.write.Fd()), []byte{1})
	s.pending[s.state][id] = nil
	if e != nil {
		return e
	}

	return nil
}

// what ever method calls this should call  clear on the returned map when done processing.
func (s *Worker) getChanges() (m map[int64]any, e error) {
	s.locker.RLock()
	defer s.locker.RUnlock()
	_, e = unix.Read(int(s.read.Fd()), s.buffer)
	state := s.state
	// alternate between odd and even
	s.state = (state + 1) & 1
	m = s.pending[state]
	return
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

	_, err = unix.Write(int(s.write.Fd()), []byte{1})
	if err != nil {
		s.write.Close()
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

func (s *Worker) SingleRun() error {
	if s.closed {
		// if we are in shutdown mode.. well nothing to see here.. move along!
		return ERR_SHUTDOWN
	}
	sleep := s.nextState()

	active, e := s.doPoll(sleep)
	if e != nil {
		return e
	}
	s.now = time.Now()
	s.processNextSet(active)
	if s.closed {
		return ERR_SHUTDOWN
	}

	return nil
}

func (s *Worker) nextState() (sleep int64) {

	now := time.Now().UnixMilli()
	sleep = -1
	if ts, ok := s.timeouts.FirstKey(); ok {
		diff := ts - now
		sleep = max(diff, 0)
		now += sleep
	}
	return
}

func (s *Worker) doPoll(sleep int64) (active int, err error) {

	if s.closed {
		err = ERR_SHUTDOWN
		return
	}

	if s.limit == 0 {
		total := len(s.fdjobs)
		if cap(s.events) < total {
			s.events = slices.Grow(s.events, total)
			s.events = s.events[:total]

		}
	}

	active, err = unix.EpollWait(s.epfd, s.events, int(sleep))
	return
}

func (s *Worker) processNextSet(active int) {

	now := s.now.UnixMilli()
	for i := range active {
		events := s.events[i].Events
		fd := s.events[i].Fd

		job, _ := s.fdjobs[fd]

		check := events & job.wanted
		if check == 0 {
			job.InEventLoop()
			// if we get here, we need to check for errors dirrectly
			if events&IN_EOF != 0 {
				// best guess is EOF.. may be a few others
				// like io.ErrClosedPipe
				s.clearJob(job, io.EOF)
			} else if events&IN_ERROR != 0 {
				s.clearJob(job, io.ErrUnexpectedEOF)
			}
			// start our error check states
			continue

		}
		w, t, e := job.ProcessEvents(check, now)
		if e != nil || (w == 0 && t <= 0) {
			s.clearJob(job, e)
			continue
		}

		s.resetTimeout(job, t)
		s.changeEvents(job, w)
	}

	for lastTimeout, jobs := range s.timeouts.RemoveBetweenKV(-1, now, omap.FIRST_KEY) {
		// side effect of entering this loop, each delement in this array has been deleted.
		for id, job := range jobs {
			w, t, e := job.CheckTimeOut(now, lastTimeout)
			jc := s.jobs[id]
			if e != nil || (w == 0 && t <= 0) {
				s.clearJob(jc, e)
				continue
			}
			s.updateTimeout(jc, t)
			s.changeEvents(jc, w)
		}
	}
}

func (s *Worker) changeEvents(job *wjc, events uint32) bool {
	fd := job.Fd()
	if fd > -1 && job.wanted != events {
		// update what we want!
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
		job.wanted = events
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
	if s.limit != 0 && s.ctrl.jobId != job.JobId() {
		<-s.throttle
	}
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

func (s *Worker) updateTimeout(job *wjc, ts int64) (needsCleanup bool) {
	if ts == job.nextTs {
		return false
	}

	needsCleanup = job.nextTs > 0
	if ts < 1 {
		job.nextTs = ts
		// stop here if we have nothing to do
		return
	}
	var m map[int64]Job
	var ok bool
	if m, ok = s.timeouts.Get(ts); !ok {
		m = make(map[int64]Job)
		s.timeouts.Put(ts, m)
	}
	job.nextTs = ts
	m[job.JobId()] = job
	return
}

func (s *Worker) resetTimeout(job *wjc, t int64) {
	if lt := job.nextTs; s.updateTimeout(job, t) {
		// only clear the old job if we need to
		if lt != job.nextTs {
			if m, ok := s.timeouts.Get(lt); ok {
				delete(m, job.JobId())
				if len(m) == 0 {
					s.timeouts.Remove(lt)
				}
			}
		}
	}
}

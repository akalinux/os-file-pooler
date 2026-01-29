package osfp

import (
	"cmp"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"slices"
	"sync"
	"sync/atomic"
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

var WorkerId *uint64

func init() {
	var id uint64 = 0
	WorkerId = &id
}

type Worker struct {
	WorkerID uint64
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
	wg       *sync.WaitGroup
	running  bool
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
		nil,
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

func NewWorker(que chan Job, throttle chan any, read *os.File, write *os.File, limit int, wg *sync.WaitGroup) (*Worker, error) {

	// create in level not edge mode!
	epfd, e := unix.EpollCreate1(0)
	if e != nil {
		write.Close()
		return nil, e
	}
	s := &Worker{
		throttle: throttle,
		que:      que,
		WorkerID: atomic.AddUint64(WorkerId, 1),
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
		wg:     wg,
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
	fmt.Printf("Buffer size is: %d\n", len(s.buffer))
	defer func() {

		s.locker.RUnlock()
		fmt.Printf("Finished Getting changes \n")
	}()

	//n, e := unix.Read(int(s.read.Fd()), s.buffer)
	n, e := s.read.Read(s.buffer)
	fmt.Printf("got :%d bytes\n", n)
	if n == 0 {
		e = ERR_SHUTDOWN
		return
	}
	if e != nil {
		fmt.Printf("getting here\n")
		return
	}
	fmt.Printf("Got past read\n")
	state := s.state
	// alternate between odd and even
	s.state = (state + 1) & 1
	m = s.pending[state]
	return
}

func (s *Worker) LocalPoolUsage() (usage, limit int) {
	fmt.Printf("Getting pool info\n")
	s.locker.RLock()

	defer s.locker.RUnlock()
	defer fmt.Printf("Finished Getting pool info\n")
	return s.localPoolUsage()
}

func (s *Worker) localPoolUsage() (usage, limit int) {
	usage = len(s.jobs) - 1
	limit = s.limit
	return
}

func (s *Worker) Wakeup() (err error) {
	fmt.Printf("Wakup called\n")
	s.locker.Lock()
	defer s.locker.Unlock()
	defer fmt.Printf("Finished Wakup called\n")
	err = s.wakeup()
	return

}

func (s *Worker) wakeup() (err error) {
	if s.closed {
		return ERR_SHUTDOWN
	}

	fmt.Printf("Wriitng 1 byte\n")
	_, err = unix.Write(int(s.write.Fd()), []byte{1})
	if err != nil {
		fmt.Printf("Someting went wrong? :%v\n", err)
		s.write.Close()
	}
	return
}

func (s *Worker) Start() error {

	if s.closed {
		return ERR_SHUTDOWN
	}
	if s.wg != nil {
		slog.Info(fmt.Sprintf("Starting worker: %d", s.WorkerID))
	}

	s.toggleRunning(true)
	for !s.closed {

		fmt.Printf("running again\n")
		s.SingleRun()
	}
	defer s.toggleRunning(false)

	return nil
}

func (s *Worker) toggleRunning(r bool) {
	fmt.Println("Toggling running")
	s.locker.Lock()
	defer s.locker.Unlock()
	defer fmt.Println("Finished Toggling running")
	s.running = r
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

	if s.closed {
		return nil
	}
	s.now = time.Now()
	s.processNextSet(active)

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

func (s *Worker) IsClosed() bool {
	s.locker.RLock()
	defer s.locker.RUnlock()
	return s.closed
}
func (s *Worker) processNextSet(active int) {

	now := s.now.UnixMilli()
	fmt.Printf("We have acvtive: %d, and  jobs: %d, closed: %v\n", active, len(s.fdjobs), s.closed)

	if active != 0 {

		for i := range active {
			events := s.events[i].Events
			fd := s.events[i].Fd

			job, ok := s.fdjobs[fd]
			if !ok {
				// must be in shutdown mode
				if !s.IsClosed() {
					slog.Error("Got an invalid job while not not closed")
				}
				return
			}
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
			w, t, e := job.ProcessEvents(events, now)
			if e != nil || (w == 0 && t <= 0) {
				s.clearJob(job, e)
				continue
			}

			s.resetTimeout(job, t)
			s.changeEvents(job, w)
		}
	}

	if s.IsClosed() {
		return
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
	s.locker.RLock()

	if s.closed {
		s.locker.RUnlock()
		return ERR_SHUTDOWN
	}
	fmt.Printf("Finished trying to run close\n")
	s.locker.RUnlock()
	s.write.Close()
	// if we get here.. then we need to secure the write lock
	fmt.Printf("We need a real lock\n")
	s.locker.Lock()
	s.closed = true

	defer s.locker.Unlock()

	if s.running {
		//TODO
	}
	s.timeouts.RemoveAll()
	for _, job := range s.jobs {
		if job.Fd() > -1 {
			unix.EpollCtl(s.epfd, unix.EPOLL_CTL_DEL, int(job.Fd()), nil)
		}
		job.InEventLoop()
		job.ClearPool(ERR_SHUTDOWN)
	}
	unix.EpollCtl(s.epfd, unix.EPOLL_CTL_DEL, int(s.ctrl.Fd()), nil)
	unix.Close(s.epfd)
	close(s.que)
	if s.throttle != nil {
		close(s.throttle)
	}
	s.write.Close()
	if s.wg != nil {
		slog.Info(fmt.Sprintf("Pool Control Job: Shut down worker: %d", s.WorkerID))
		s.wg.Done()
	}
	return nil
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

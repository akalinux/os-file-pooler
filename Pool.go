package osfp

import (
	"errors"
	"os"
	"sync"
)

type Pool struct {
	locker   sync.Mutex
	workers  []*Worker
	closed   bool
	throttle chan any
	que      chan Job
}

func (s *Pool) AddJob(job Job) error {
	s.locker.Lock()
	defer s.locker.Unlock()
	if s.closed {
		return errors.New("Pool is closed!")
	}

	// force blocking here if the throttle que is backed up
	s.throttle <- struct{}{}

	s.que <- job
	for _, worker := range s.workers {
		// if this runs into an error.. most likely its a memory limit
		worker.Wakeup()
	}

	return nil
}

func (s *Pool) Close() error {
	s.locker.Lock()
	defer s.locker.Unlock()
	if s.closed {
		return errors.New("Pool is all ready closed!")
	}
	for _, worker := range s.workers {
		worker.Close()
	}
	return nil
}

const (
	DEFAULT_POOL_THREADS   = 10
	DEFAULT_POOL_HANDLES   = 1000
	DEFAULT_WORKER_HANDLES = DEFAULT_POOL_HANDLES / DEFAULT_POOL_THREADS
)

// Creates a new Pool instance with the default number of threads and limits.
func NewPoolDefaults() (*Pool, error) {
	return NewPool(DEFAULT_POOL_THREADS, DEFAULT_POOL_HANDLES)
}

func NewPool(threads int, limit int) (*Pool, error) {
	if threads == 0 {
		return nil, errors.New("threads cannot be 0")
	}
	if limit < threads {
		return nil, errors.New("limit must be greater than threads")
	}

	if threads%limit != 0 {
		return nil, errors.New("limit must be a multiple of threads")
	}
	que := make(chan Job, threads)
	throttle := make(chan any, limit)
	workers := make([]*Worker, 0, threads)
	sigs := make([]*os.File, 0, threads)

	// we always have 1 master file handle
	memberLimit := 1 + limit/threads
	res := Pool{
		workers:  workers,
		throttle: throttle,
		que:      que,
	}

	for i := range threads {
		r, w, e := os.Pipe()
		if e != nil {
			res.Close()
			return nil, e
		}
		t := NewWorker(que, throttle, r, w, memberLimit)
		go t.Run()
		workers[i] = t
		sigs[i] = w
	}

	return &res, nil
}

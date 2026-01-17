package osfp

import (
	"cmp"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"slices"
	"sync"
)

var ERR_IS_RUNNING = errors.New("Thread pool is all ready running")

type Pool struct {
	locker   sync.RWMutex
	workers  []*Worker
	closed   bool
	running  bool
	throttle chan any
	que      chan Job
}

func (s *Pool) Stop() error {
	s.locker.Lock()
	defer s.locker.Unlock()
	if s.closed || !s.running {
		return ERR_SHUTDOWN
	}
	s.running = false

	for _, worker := range s.workers {
		worker.Stop()
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

	// we always have 1 master file handle
	memberLimit := limit / threads
	res := Pool{
		workers:  workers,
		throttle: throttle,
		que:      que,
	}

	for i := range threads {
		r, w, e := os.Pipe()
		if e != nil {
			res.Stop()
			return nil, e
		}
		t, e := NewWorker(que, throttle, r, w, memberLimit)
		if e != nil {
			res.Stop()
			w.Close()
			return nil, e
		}

		go t.Start()
		workers[i] = t
	}

	return &res, nil
}

type sortSet struct {
	worker int
	jobs   int
}

func cmpSortSet(a, b *sortSet) int {
	return cmp.Compare(a.jobs, b.jobs)
}

// Adds a Job to the pool, does best effort on load ballancing.
func (s *Pool) AddJob(job Job) error {
	s.locker.RLock()
	defer s.locker.RUnlock()
	if s.closed || !s.running {
		return ERR_SHUTDOWN
	}
	s.throttle <- struct{}{}
	s.que <- job

	snapshot := make([]*sortSet, len(s.workers))
	for i, worker := range s.workers {
		snapshot[i] = &sortSet{worker: i, jobs: worker.JobCount()}
	}
	slices.SortFunc(snapshot, cmpSortSet)
	for _, set := range snapshot {
		s.workers[set.worker].Wakeup()
	}

	return nil
}

func (s *Pool) Start() error {
	s.locker.Lock()
	defer s.locker.Unlock()
	slog.Info("Trying to start worker pool")
	if s.closed {
		return ERR_SHUTDOWN
	}
	if s.running {
		return ERR_IS_RUNNING
	}
	s.running = true

	for i := range s.workers {
		slog.Info(fmt.Sprintf("Starting thread %d", i))
		go s.launchWorker(i)
	}
	return nil
}

func (s *Pool) launchWorker(i int) {
	slog.Info(fmt.Sprintf("Starting worker: %d", i))
	w := s.workers[i]
	w.Start()
	slog.Info(fmt.Sprintf("Stopping worker: %d", i))
	defer s.workerPanic(w, i)
}

func (s *Pool) workerPanic(worker *Worker, id int) {
	if e := recover(); e != nil {
		msg := fmt.Sprintf("Thread Pool Panic in worker: %d, eror was: %v", id, e)
		slog.Error(msg)
		worker.Stop()
	}
}

func (s *Pool) NewUtil() *Util {
	return &Util{s}
}

package osfp

import (
	"errors"
	"os"
	"sync"

	// may be able to do this in the fute
	//"syscall"
	"golang.org/x/sys/unix"
)

type Pool struct {
	locker   sync.Mutex
	pools    []*Worker
	sigs     []*os.File
	closed   bool
	throttle chan any
	que      chan *Job
}

// Creates a new unix pipe as os.File.
func Pipe() (r *os.File, w *os.File, e error) {
	fd := [2]int{}
	//err := syscall.Pipe(fd[:])
	err := unix.Pipe(fd[:])
	if err != nil {
		return nil, nil, err
	}
	return os.NewFile(uintptr(fd[0]), "Read Pipe"),
		os.NewFile(uintptr(fd[1]), "Write Pipe"),
		nil
}

func (s *Pool) Close() {
	if s.closed {
		return
	}
	for tid, worker := range s.pools {
		worker.file.Close()
		s.sigs[tid].Close()
	}

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
	que := make(chan *Job, threads)
	throttle := make(chan any, limit)
	pools := make([]*Worker, 0, threads)
	sigs := make([]*os.File, 0, threads)
	memberLimit := limit / threads
	res := Pool{
		pools:    pools,
		throttle: throttle,
		que:      que,
		sigs:     sigs,
	}
	for i := range threads {
		r, w, e := Pipe()
		if e != nil {
			res.Close()
			return nil, e
		}
		t := NewWorker(que, throttle, r, memberLimit)
		go t.Run()
		pools[i] = t
		sigs[i] = w
	}

	return &res, nil
}

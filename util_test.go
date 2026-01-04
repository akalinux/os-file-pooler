package osfp

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

const TEST_STRING = "this is a test"

type WorkerTestSet struct {
	r      *os.File
	w      *os.File
	Job    *CallBackJob
	Worker *Worker
}

func (s *Worker) getWorkerDebuStates() (fdc, fdn, jobc, jobn int, state byte) {
	if s.fds[0] == s.fds[1] {
		panic("Fds current and next are the same instance!")
	}
	if s.jobs[0] == s.jobs[1] {
		panic("Fds current and next are the same instance!")
	}
	fdc = len((*s.fds[0]))
	fdn = len((*s.fds[1]))
	jobc = len((*s.jobs[0]))
	jobn = len((*s.jobs[1]))
	state = s.state
	return
}

func (s *Worker) WorkerStateDebugString() string {
	cf, nf, cj, nj, state := s.getWorkerDebuStates()
	return fmt.Sprintf("State: %d, fds[0]:%d, fds[1];%d, jobs[0]:%d, jobs[1]:%d", state, cf, nf, cj, nj)
}

func (s *WorkerTestSet) WorkerCleanup() {
	s.w.Close()
	s.Worker.Close()
}

// For internal use only!!
// This method exists mostly to make unit testing easier
func (s *Worker) forceAddJobToWorker(job Job) (futureTimeOut int64) {
	s.throttle <- struct{}{}
	now := time.Now().UnixMilli()
	var watchEevents int16
	var fd int32
	watchEevents, futureTimeOut, fd = job.SetPool(s, now)
	p := unix.PollFd{Fd: fd, Events: watchEevents}
	for i := range 2 {
		jobs := s.jobs[i]
		*jobs = append(*jobs, &JobContainer{Job: job})
		fds := s.fds[i]
		*fds = append(*fds, p)
	}
	return
}

func Pipe() (r, w *os.File) {
	var e error
	r, w, e = os.Pipe()
	if e != nil {
		panic(e)
	}
	return
}

var wokerCount = 1

func createLocalWorker() *Worker {
	// always have the master job.. so to run more than
	w, e := NewLocalWorker(wokerCount)
	if e != nil {
		// if this breaks.. ya no point in testing anyting else!
		panic(e)
	}
	return w
}

func createRJob(cb func(config *OnCallBackConfig)) (job Job, r *os.File, w *os.File) {
	var e error
	r, w, e = os.Pipe()
	if e != nil {
		panic(e)
	}
	job = NewJobFromFdT(int32(r.Fd()), CAN_READ, 0, cb)
	return
}

func (s *WorkerTestSet) singleLoop(t *testing.T) {

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	fail := true

	go func() {
		s.Worker.SingleRun()
		fail = false
		defer cancel()
	}()

	<-ctx.Done()
	if fail {
		t.Fatal("Something went worng, the event loop did not run!")
		panic("Single worker pass never ran.. ")
	}
}
func spawnRJobAndWorker(t *testing.T) *WorkerTestSet {
	s := createLocalWorker()

	job, r, w := createRJob(nil)
	e := s.AddJob(job)
	if e != nil {
		panic("Failed to add job, error was: " + e.Error())
	}

	rj, _ := job.(*CallBackJob)

	res := WorkerTestSet{
		r:      r,
		w:      w,
		Worker: s,
		Job:    rj,
	}
	res.singleLoop(t)
	if s.JobCount() != 1 {
		res.WorkerCleanup()
		panic("Job was not picked up by internals")
	}
	return &res
}

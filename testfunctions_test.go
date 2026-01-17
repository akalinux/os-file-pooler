package osfp

import (
	"context"
	"os"
	"testing"
	"time"
)

const TEST_STRING = "this is a test"

type WorkerTestSet struct {
	r      *os.File
	w      *os.File
	Job    *CallBackJob
	Worker *Worker
}

func (s *WorkerTestSet) WorkerCleanup() {
	s.w.Close()
	s.Worker.Stop()
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

func createRJob(cb func(config *CallbackEvent)) (job Job, r *os.File, w *os.File) {
	var e error
	r, w, e = os.Pipe()
	if e != nil {
		panic(e)
	}
	job = NewJobFromOsFileT(r, CAN_READ, 0, cb)
	return
}

func createWJob(cb func(config *CallbackEvent)) (job Job, r *os.File, w *os.File) {
	var e error
	r, w, e = os.Pipe()
	if e != nil {
		panic(e)
	}
	job = NewJobFromOsFileT(w, CAN_RW, 0, cb)
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

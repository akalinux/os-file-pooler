package osfp

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"
)

func TestNewWorker(t *testing.T) {

	w, e := NewStandAloneWorker()
	if e != nil {
		t.Errorf("Failed to start New local worker")
		return
	}

	if w.JobCount() != 0 {
		t.Errorf("Expected 0 elements, got: %d", w.JobCount())
	}

	// should not throw errors
	w.Close()
	w.Close()
	w.Close()
}

func TestNextState(t *testing.T) {
	s := createLocalWorker()
	defer s.Close()
	lstate := s.state
	job, _, w := createRJob(nil)
	s.forceAddJobToWorker(job)
	defer w.Close()
	currentState, nextState, now, sleep := s.NextState()
	if lstate != currentState {
		t.Error("current state, should match state")
		return
	}
	if s.state != nextState {
		t.Errorf("internal state should now be nextState, but it is not?? got: %d, expected: %d", s.state, nextState)
		return
	}

	if now <= 0 {
		t.Error("Now should be curent time in ms???")
		return
	}

	if sleep != -1 {
		t.Error("inital sleep should always be -1")
	}
	// Check current slice sizes
	fdc, fdn, jobc, jobn, _ := s.getWorkerDebuStates()
	t.Log(s.WorkerStateDebugString())
	if fdc != 2 {
		t.Fatalf("bad fdc size, Expected: 2 ,got: %d", fdc)
	}
	if fdn != 1 {
		t.Fatalf("bad fdn size, Expected: 1 ,got: %d", fdn)
	}
	if jobc != 2 {
		t.Fatalf("bad jobc size, Expected: 2 ,got: %d", jobc)
	}
	if jobn != 1 {
		t.Fatalf("bad jobn size, Expected: 1 ,got: %d", jobn)
	}

}

func TestAddJob(t *testing.T) {
	t.Log("starting TestAddJob")
	t.Log("Creating worker")
	s := createLocalWorker()
	t.Log("Creating Job")
	if s.JobCount() != 0 {
		panic("Internals should show 0 jobs!")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	var job Job
	var w *os.File

	job, _, w = createRJob(nil)
	t.Log("Job created")
	defer w.Close()
	defer cancel()
	defer s.Close()
	t.Log("Adding Job")
	err := s.TryAddJob(job)
	if err != nil {
		panic("We should be able to add our job")
	}
	t.Log("Job Added")

	go func() {
		defer cancel()
		t.Log("Starting poll test")
		s.nextTs = time.Now().UnixMilli() + time.Hour.Milliseconds()*500
		s.SingleRun()
		t.Logf("Job Count: %d", s.JobCount())
		t.Log("poll test completed")
	}()

	<-ctx.Done()

	if s.JobCount() != 1 {

		cf, nf, cj, nj, state := s.getWorkerDebuStates()

		t.Fatalf("Job count should be 1.. if not something went wrong!\n"+
			"  State: %d\n  fds[0]:%d, fds[1];%d jobs[0]:%d, jobs[1]:%d\n", state, cf, nf, cj, nj)
	}
	t.Log(s.PoolUsageString())
	err = s.TryAddJob(job)
	if err == nil {
		t.Fatalf("Should have failed to add")
	}

}

func TestClearJob(t *testing.T) {
	w := spawnRJobAndWorker(t)
	w.w.Close()
	var e error
	defer w.WorkerCleanup()
	w.Job.OnEvent = func(c *OnCallBackConfig) {
		e = c.InError()
	}
	w.singleLoop(t)
	if e == nil {
		t.Fatalf("Did not get our EOF event?")
	}
	t.Log(w.Worker.WorkerStateDebugString())

}

func TestWorkerJobRead(t *testing.T) {
	w := spawnRJobAndWorker(t)
	c := ""
	b := make([]byte, 2)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer w.WorkerCleanup()
	defer cancel()
	w.Job.OnEvent = func(config *OnCallBackConfig) {
		t.Log("*** GOT READ EVENT ***")

		if config.InError() != nil {
			if config.InError() != ERR_SHUTDOWN {
				panic("Non shutdown error!")
			}
			t.Log("Got Shutdown error")
			return
		}

		s, x := w.r.Read(b[:cap(b)])
		if x != nil {
			panic(x)
		}
		c += string(b[:s])
		if c == TEST_STRING {
			t.Log("Got full string")
			w.Worker.Close()
		}
	}

	fail := true
	go func() {
		// full test
		w.Worker.Run()
		fail = false
		cancel()
	}()
	w.w.Write([]byte(TEST_STRING))
	<-ctx.Done()
	if c != TEST_STRING {
		t.Error("Did not read full string")
	}
	if fail {
		t.Fatalf("Close of worker failed!")
	}

}

func TestWorkerUpdateTimeout(t *testing.T) {
	w := spawnRJobAndWorker(t)
	w.Job.SetTimeout(5)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer w.WorkerCleanup()
	defer cancel()
	var timeout bool
	var diff int64 = 0
	now := time.Now().UnixMilli()
	var e error
	w.Job.OnEvent = func(config *OnCallBackConfig) {
		if timeout, e = config.InTimeout(); timeout {
			diff = time.Now().UnixMilli() - now
			t.Log("Timeout Check completed")
		} else if e != nil {
			t.Log("Error Check Completed, Shutting the worker down")
			w.Worker.Close()
		}
	}
	t.Logf("%s\n", w.Worker.WorkerStateDebugString())
	go func() {
		w.Worker.Run()
		defer cancel()
	}()
	<-ctx.Done()

	t.Logf("Our diff was: %d", diff)
	t.Logf("%s\n", w.Worker.WorkerStateDebugString())
	if diff == 0 {
		t.Error("Time diff is zero")
	}
	if e != os.ErrDeadlineExceeded {
		t.Fatalf("Never got our timeout")
	}
}

func TestAddTimeout(t *testing.T) {
	worker := createLocalWorker()
	j, _, w := createRJob(nil)
	job, _ := j.(*CallBackJob)
	job.SetTimeout(5)
	defer worker.Close()
	defer w.Close()
	var timeout bool
	var diff int64 = 0
	now := time.Now().UnixMilli()
	var e error
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	job.OnEvent = func(config *OnCallBackConfig) {
		if timeout, e = config.InTimeout(); timeout {
			diff = time.Now().UnixMilli() - now
			t.Log("Timeout Check completed")
		} else if e != nil {
			t.Log("Error Check Completed, Shutting the worker down")
			worker.Close()
		}
	}
	worker.AddJob(job)
	go func() {
		worker.Run()
		defer cancel()
	}()
	<-ctx.Done()

	t.Logf("Our diff was: %d", diff)
	t.Logf("%s\n", worker.WorkerStateDebugString())
	if diff == 0 {
		t.Error("Time diff is zero")
	}
	if e != os.ErrDeadlineExceeded {
		t.Fatalf("Never got our timeout")
	}

}

func TestMultipleTimeouts(t *testing.T) {
	w := func() *WorkerTestSet {
		wokerCount = 3
		defer func() {
			wokerCount = 1
		}()
		return spawnRJobAndWorker(t)
	}()
	defer func() { wokerCount = 1 }()
	t.Logf("Pool Usage: %s", w.Worker.PoolUsageString())
	w.Job.SetTimeout(5)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer w.WorkerCleanup()
	defer cancel()

	jobs := [3]*CallBackJob{}
	jobs[0] = w.Job

	ja, _, fdwa := createRJob(nil)
	jb, _, fdwb := createRJob(nil)
	defer fdwa.Close()
	defer fdwb.Close()
	t.Log("Adding Job 2")
	w.Worker.AddJob(ja)
	t.Log("Adding Job 3")
	w.Worker.AddJob(jb)

	jobs[1] = ja.(*CallBackJob)
	jobs[2] = jb.(*CallBackJob)

	results := [3]map[string]any{
		make(map[string]any),
		make(map[string]any),
		make(map[string]any),
	}
	var offset int64 = 10
	var size int64 = 60
	var cmp int64 = size
	var wg sync.WaitGroup
	for i := range 3 {
		wg.Add(1)
		limit := cmp - offset
		jobs[i].SetTimeout(limit)
		cmp = limit
		jobs[i].OnEvent = func(e *OnCallBackConfig) {
			if ok, err := e.InTimeout(); ok {
				results[i]["ok"] = ok
			} else {
				wg.Done()
				results[i]["end"] = time.Now().UnixMilli()
				results[i]["err"] = err
			}
		}
	}

	t.Log("Starting Loop")
	now := time.Now().UnixMilli()
	go func() {
		// test framework created 1 job, the other 2 will be added
		for w.Worker.JobCount() != 0 {
			t.Logf("Job Count %d", w.Worker.JobCount())
			w.Worker.SingleRun()
		}
		t.Logf("Job Count %d", w.Worker.JobCount())
		cancel()
	}()

	<-ctx.Done()
	cmp = size
	for i, res := range results {
		if len(res) != 3 {
			t.Errorf("Job: %d, did not complete", i)
			continue
		}
		v, _ := res["end"]
		end, _ := v.(int64)
		diff := now - end

		t.Logf("Diff for Job: %d, was: %d", i, diff)
		if diff >= cmp {
			t.Errorf("Failed job: %d, Expected: %d < %d", i, diff, cmp)
		}
		cmp -= offset

	}
}

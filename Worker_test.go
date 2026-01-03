package osfp

import (
	"context"
	"os"
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
	w.Job.OnEvent = func(_ *OnCallBackConfig, E error) {
		e = E
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
	w.Job.OnEvent = func(config *OnCallBackConfig, e error) {
		t.Log("*** GOT READ EVENT ***")

		if e != nil {
			if e != ERR_SHUTDOWN {
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

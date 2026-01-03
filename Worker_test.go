package osfp

import (
	"os"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func Pipe() (r, w *os.File) {
	var e error
	r, w, e = os.Pipe()
	if e != nil {
		panic(e)
	}
	return
}

func createLocalWorker() *Worker {

	w, e := NewStandAloneWorker()
	if e != nil {
		// if this breaks.. ya no point in testing anyting else!
		panic(e)
	}
	return w
}
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

func createRJob(cb func(int16, error)) (job Job, r *os.File, w *os.File) {
	var e error
	r, w, e = os.Pipe()
	if e != nil {
		panic(e)
	}
	job = NewJobFromFdT(int32(r.Fd()), CAN_READ, 0, cb)
	return
}

// For internal use only!!
// This method exists mostly to make unit testing easier
func (s *Worker) forceAdd(job Job) (futureTimeOut int64) {
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

func (s *Worker) getStates() (fdc, fdn, jobc, jobn int, state byte) {
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
func TestNextState(t *testing.T) {
	s := createLocalWorker()
	defer s.Close()
	lstate := s.state
	currentState, nextState, now, sleep := s.NextState()
	job, _, w := createRJob(nil)
	defer w.Close()
	s.forceAdd(job)
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
	// states should be swapped, so "n"=="c" and "c"=="n"
	fdc, fdn, jobc, jobn, _ := s.getStates()
	if fdc != 1 {
		*s.fds[1] = (*s.fds[1])[:1]
		check := len(*s.fds[1])
		t.Errorf("bad fdc size, Expected: 1 ,got: %d, sanity check: %d", fdc, check)
	}
	if fdn != 2 {
		t.Errorf("bad fdn size, Expected: 2 ,got: %d", fdn)
	}
	if jobc != 1 {
		t.Errorf("bad jobc size, Expected: 1 ,got: %d", jobc)
	}
	if jobn != 2 {
		t.Errorf("bad jobn size, Expected: 2 ,got: %d", jobn)
	}

}

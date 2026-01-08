package osfp

import (
	"context"
	"os"
	"os/exec"
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

	// Synthetic Job
	ph, _, f := createRJob(nil)
	defer f.Close()
	job, _ := ph.(*CallBackJob)
	job.events = 0
	w.throttle <- struct{}{}
	w.que <- job
	w.write.Write([]byte{1})
	(*w.jobs[0])[0].ProcessEvents(0, 0)

	// code coveratge.. for noop function
	(*w.jobs[0])[0].CheckTimeOut(0, 0)

	// should not throw errors
	e = w.Stop()
	(*w.jobs[0])[0].ProcessEvents(0, 0)
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)

	go func() {
		w.SingleRun()
		cancel()
	}()
	<-ctx.Done()

	e = w.Stop()
	if e != ERR_SHUTDOWN {
		t.Fatalf("Should have gotten a shutdown error!")
	}
	e = w.AddJob(nil)
	if e != ERR_SHUTDOWN {
		t.Fatalf("Should have gotten a shutdown error!")
	}

	e = w.Wakeup()
	if e != ERR_SHUTDOWN {
		t.Fatalf("Should have gotten a shutdown error!")
	}
	if e = w.SingleRun(); e != ERR_SHUTDOWN {
		t.Fatalf("Should have gotten a shutdown error!")
	}

	// will crash if the code is broken..
	(*w.jobs[0])[0].ProcessEvents(0, 0)

}
func TestSimulateOsError(t *testing.T) {
	w, e := NewStandAloneWorker()
	defer func() { w.Stop() }()
	w.write.Close()
	e = w.Wakeup()
	if e == nil {
		t.Fatalf("Os simulation failed?")
	}
}

func TestNextState(t *testing.T) {
	s := createLocalWorker()
	defer s.Stop()
	lstate := s.state
	job, _, w := createRJob(nil)
	s.forceAddJobToWorker(job)
	defer w.Close()
	currentState, nextState, now, sleep := s.nextState()
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
	defer s.Stop()
	t.Log("Adding Job, worker State is: " + s.PoolUsageString())
	err := s.AddJob(job)
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
	err = s.AddJob(job)
	if err == nil {
		t.Fatalf("Should have failed to add")
	}

}

func TestClearJob(t *testing.T) {
	w := spawnRJobAndWorker(t)
	w.w.Close()
	var e error
	defer w.WorkerCleanup()
	w.Job.onEvent = func(c *CallbackEvent) {
		e = c.Error()
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

	w.Job.timeout = 50
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer w.WorkerCleanup()
	defer cancel()
	w.Job.onEvent = func(config *CallbackEvent) {
		t.Log("*** GOT READ EVENT ***")

		if config.Error() != nil {
			if config.Error() != ERR_SHUTDOWN {
				panic("Non shutdown error!")
			}
			t.Log("Got Shutdown error")
			return
		}
		config.PollRead()

		if !config.IsRead() {
			panic("SHOULD BE ABLE TO READ")
		}
		if config.IsWrite() {
			panic("We are not in a write mode!")
		}
		if config.IsRW() {
			panic("We are not in a Read+Write mode!")
		}
		s, x := w.r.Read(b[:cap(b)])
		t.Logf("Timeout is: %d", w.Job.timeout)
		if x != nil {
			panic(x)
		}
		c += string(b[:s])
		if c == TEST_STRING {
			t.Log("Got full string")
			w.Worker.Stop()
		}
	}

	fail := true
	go func() {
		// full test
		w.Worker.Start()
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
	w.Job.onEvent = func(config *CallbackEvent) {
		if timeout = config.InTimeout(); timeout {
			e = config.Error()
			diff = time.Now().UnixMilli() - now
			t.Log("Timeout Check completed")
			t.Log("Error Check Completed, Shutting the worker down")
			w.Worker.Stop()
		}
	}
	t.Logf("%s\n", w.Worker.WorkerStateDebugString())
	go func() {
		w.Worker.Start()
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

func TestAddFdTimeout(t *testing.T) {
	worker := createLocalWorker()
	j, _, w := createRJob(nil)
	job, _ := j.(*CallBackJob)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)

	// force the worker to wake up
	worker.Wakeup()

	job.SetTimeout(5)
	defer worker.Stop()
	defer w.Close()
	var timeout bool
	var diff int64 = 0
	now := time.Now().UnixMilli()
	var e error
	ctx, cancel = context.WithTimeout(context.Background(), 2*time.Second)
	job.onEvent = func(config *CallbackEvent) {
		if timeout = config.InTimeout(); timeout {
			diff = time.Now().UnixMilli() - now
			t.Log("Timeout Check completed")
			t.Log("Error Check Completed, Shutting the worker down")
			e = config.Error()
			worker.Stop()
		}
	}
	worker.AddJob(job)
	go func() {
		worker.Start()
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

func TestMultipleFdTimeouts(t *testing.T) {
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
	for i := range 3 {
		limit := cmp - offset
		jobs[i].SetTimeout(limit)
		cmp = limit
		count := 0
		id := i
		jobs[i].onEvent = func(e *CallbackEvent) {
			count++
			t.Logf("** Job %d, called: %d", id, count)
			t.Logf("Config: %v\n", e)
			t.Log("In Callback")
			if ok := e.InTimeout(); ok {
				results[i]["ok"] = ok
				results[i]["err"] = e.Error()
				results[i]["end"] = time.Now().UnixMilli()
				t.Log("In Timeout")
			} else if e.Error() != nil {
				t.Log("Something is going wrong")
			} else if e.IsRead() {
				t.Log("Reading?")
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

func TestTimeout(t *testing.T) {
	w := createLocalWorker()
	defer w.Stop()
	var ts int64
	var e error
	var ok bool = false
	job := &CallBackJob{
		timeout: 25,
		onEvent: func(c *CallbackEvent) {
			if ok = c.InTimeout(); ok {
				ts = time.Now().UnixMilli()
				e = c.Error()
			}
		},
	}
	w.AddJob(job)
	t.Log("Starting Loop")
	now := time.Now().UnixMilli()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	go func() {
		count := 0
		// test framework created 1 job, the other 2 will be added
		for count == 0 || w.JobCount() != 0 {
			t.Logf("Job Count %d", w.JobCount())
			w.SingleRun()
			count++
		}
		t.Logf("Job Count %d", w.JobCount())
		cancel()
	}()
	defer cancel()

	<-ctx.Done()

	if ts < now {
		t.Error("Timeout did not run")
	}
	if e == nil {
		t.Error("Job did not finish")
	}
}

func TestWakeThenUpdateTimeout(t *testing.T) {
	w := spawnRJobAndWorker(t)
	defer w.WorkerCleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	w.Worker.Wakeup()

	go func() {
		w.Worker.SingleRun()
		cancel()
	}()
	<-ctx.Done()
	// code coverage
	w.Job.SetTimeout(0)

	w.Job.SetTimeout(5)

	count := 0
	ctx, cancel = context.WithTimeout(context.Background(), 2*time.Second)
	w.Job.onEvent = func(config *CallbackEvent) {
		t.Logf("In Callback")

		if !config.InTimeout() {
			return
		}
		if count < 2 {
			// update our timeout after our timeout
			config.SetTimeout(5)
			count++
		}
		// make sure we don't do it again

	}

	go func() {
		for count == 0 || w.Worker.JobCount() != 0 {
			t.Log("Entering Woker loop: " + w.Worker.WorkerStateDebugString())
			w.Worker.SingleRun()
		}
		t.Log("Exiting Worker loop: " + w.Worker.WorkerStateDebugString())
		cancel()
	}()

	<-ctx.Done()
	if count != 2 {
		t.Fatalf("Should have run the callback 2 times, got a total of %d", count)
	}
}

func TestRelease(t *testing.T) {
	w := spawnRJobAndWorker(t)
	defer w.WorkerCleanup()

	w.Job.onEvent = func(config *CallbackEvent) {
		config.Release()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	w.w.Write([]byte{0})

	go func() {
		w.Worker.SingleRun()
		cancel()
	}()
	<-ctx.Done()

	if w.Worker.JobCount() != 0 {
		t.Fatalf("Expected 0 jobs, got: %d", w.Worker.JobCount())
	}
}

func TestWrite(t *testing.T) {
	m := createLocalWorker()
	defer m.Stop()
	count := 0

	var job Job
	var r *os.File
	var w *os.File
	job, r, w = createWJob(func(config *CallbackEvent) {
		switch count {
		case 0:
			config.PollRead()
			t.Log("Writing first Chunk [Hello ]")
			w.Write([]byte("Hello "))
			if !config.IsWrite() {
				panic("Should be in write mode")
			}
			config.SetTimeout(1)
		case 1:
			if ok := config.InTimeout(); !ok {
			}
			t.Log("Swapping back to write poll")
			config.PollWrite()
			config.SetTimeout(0)
		case 2:
			t.Log("Writing second Chunk [Word!]")
			if !config.IsWrite() {
				panic("Should be in write mode")
			}
			w.Write([]byte("Workd!"))
		default:
			w.Close()
		}
		count++
	})
	m.AddJob(job)
	defer w.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*2)
	go func() {
		for count < 3 {
			t.Logf("On Pass: %d", count)
			m.SingleRun()
		}
		t.Log("Done")
		cancel()
	}()

	<-ctx.Done()
	//cmp := "Hello Workd!"
	save := ""
	buff := make([]byte, 1024)
	r.Read(buff)
	save += string(buff)
	t.Logf("Got chunk: [%s]", string(buff))

}

func TestNonWorkerReconfigure(t *testing.T) {
	w := spawnRJobAndWorker(t)
	defer w.WorkerCleanup()
	ctx, cancle := context.WithTimeout(context.Background(), time.Second*2)

	// code coverrage
	w.Job.SetCallback(nil)

	w.Job.Release()
	if w.Worker.JobCount() == 0 {
		panic("Should have a job right now")
	}

	go func() {
		for w.Worker.JobCount() != 0 {
			w.Worker.SingleRun()
		}
		cancle()
	}()
	<-ctx.Done()
	if w.Worker.JobCount() != 0 {
		t.Fatalf("Job was not released")
	}

	ctx, cancle = context.WithTimeout(context.Background(), time.Second*2)
	w.Worker.AddJob(w.Job)
	w.Job.SetEvents(CAN_READ)

	go func() {
		// need one loop load the job into the worker
		t.Log("Setting up Woker loop: " + w.Worker.WorkerStateDebugString())
		w.Worker.SingleRun()
		t.Log("Entering up Woker loop: " + w.Worker.WorkerStateDebugString())
		// force the job to have a wokrer before we start
		w.Job.SetEvents(CAN_END)
		for w.Worker.JobCount() != 0 {
			w.Worker.SingleRun()
		}
		t.Log("Exiting Woker loop: " + w.Worker.WorkerStateDebugString())
		cancle()
	}()

	<-ctx.Done()
	if w.Worker.JobCount() != 0 {
		t.Fatalf("Job was Not updated or did not release")
	}

	// code coverage
	w.Job.GetSettings()
	w.Job.SetEvents(CAN_END)
	w.Job.Release()
}

func TestUnlimitedWorker(t *testing.T) {
	w, _ := NewLocalWorker(0)
	defer w.Stop()
	for range UNLIMITED_QUE_SIZE {
		e := w.AddJob(&CallBackJob{})
		if e != nil {
			t.Fatalf("Should be able to add as many jobs as we like!")
		}
	}
	w.SingleRun()
	t.Log(w.PoolUsageString())
	w.Stop()
	// if somehting is not worek
	w.Start()
	w.Start()
	(*w.jobs[0])[0].Release()

}

func TestUtilTimeout(t *testing.T) {
	w, _ := NewLocalWorker(0)
	defer w.Stop()
	u := w.NewUtil()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*2)
	count := 0
	_, e := u.SetTimeout(func() { count++ }, 10)
	if e != nil {
		t.Fatalf("Failed to spawn job")
	}
	go func() {
		for count == 0 {
			w.SingleRun()

		}
		cancel()
	}()

	<-ctx.Done()

	if count != 1 {
		t.Fatalf("Expected: 1, got: %d", count)
	}

}

func TestUtilInterval(t *testing.T) {
	w, _ := NewLocalWorker(0)
	defer w.Stop()
	u := w.NewUtil()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*2)

	now := time.Now().UnixMilli()
	count := 0
	_, e := u.SetInterval(func(event *CallbackEvent) {
		count++

		next := time.Now().UnixMilli()
		diff := next - now
		now = next
		t.Logf("time diff is %d", diff)
		if count > 1 {
			t.Log("Rekeasing our timeout")
			event.Release()
		}
	}, 5)
	if e != nil {
		t.Fatalf("Failed to spawn job")
	}
	go func() {
		for count < 3 {
			w.SingleRun()
			t.Logf("Count is: %d", count)
			if w.JobCount() == 0 {
				t.Logf("Have 0 jobs left")
				break
			}
		}
		defer cancel()
	}()

	<-ctx.Done()

	if count != 2 {
		t.Fatalf("Expected: 2, got: %d", count)
	}
	if w.JobCount() != 0 {
		t.Fatalf("Expected: 0, got: %d", w.JobCount())
	}

}

func TestWaipid(t *testing.T) {
	w, _ := NewLocalWorker(0)
	defer w.Stop()
	u := w.NewUtil()
	_, e := u.WaitPid(-1, func(_ *WaitPidEvent) {})
	if e == nil {
		t.Fatalf("Should nto create an fd for pid -1")
	}

	cmd := exec.Command("sleep", "10")
	e = cmd.Start()
	if e != nil {
		t.Skipf("No sleep command")
		return
	}
	defer cmd.Process.Kill()
	closed := false
	_, err := u.WaitPid(cmd.Process.Pid, func(e *WaitPidEvent) {
		// will not wait becase we know the process has exited all ready!
		closed = true
	})
	if err != nil {
		t.Fatalf("Failed to create our job? %v", err)
	}

	noRun := false
	job, err := u.WaitPid(cmd.Process.Pid, func(e *WaitPidEvent) {
		t.Log("This watcher should never run!")
		noRun = true
	})
	if err != nil {
		t.Fatalf("Failed to create our job? %v", err)
	}
	// load our jobs
	w.SingleRun()
	w.nextTs = time.Now().UnixMilli()
	job.Release()
	job.Release()

	w.SingleRun()
	if closed {
		t.Fatalf("Expected this to be open")
	}

	cmd.Process.Kill()
	w.SingleRun()

	if !closed {
		t.Fatalf("Expected this to be closed")
	}
	if noRun {
		t.Fatalf("Watcher was not propery released!")
	}

}

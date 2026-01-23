package osfp

import (
	"context"
	"os"
	"os/exec"
	"testing"
	"time"
)

func TestLocalConversion(t *testing.T) {
	var i int64
	for i = -2; i < 255; i++ {

		if check := bytesToInt64(int64ToBytes(i)); check != i {
			t.Fatalf("Expected; %d, got %d", i, check)
		}
	}

}

func TestCtrlJobByteReader(t *testing.T) {
	buff := int64ToBytes(-1)
	s := &controlJob{
		buffer:  []byte{buff[0]},
		backlog: make([]byte, 0, INT64_SIZE),
	}
	var check int64
	var count int64
	onInt := func(c int64) {
		count++
		check += 0
	}
	s.byteReader(1, onInt)
	if check+count != 0 {
		t.Fatalf("Count and Check, should be 0")
	}
	if size := len(s.backlog); size != 1 {
		t.Fatalf("Expected backlog buffer to be: 1, got %d", size)
	}
	s.buffer = buff[1:INT64_SIZE]
	s.byteReader(7, onInt)
	if size := len(s.backlog); check != -1 && count != 1 && size == 0 {
		t.Fatalf("Expected Count: 1, got: %d, Expected: int -1, got: %d, expected size: 0, got: %d", count, check, size)
	}

	buff = append(buff, int64ToBytes(255)...)
	s.buffer = buff
	s.byteReader(16, onInt)
	if size := len(s.backlog); check != 253 && count != 3 && size == 0 {
		t.Fatalf("Expected Count: 3, got: %d, Expected: int 253, got: %d, expected size: 0, got: %d", count, check, size)
	}
	s.byteReader(15, onInt)
	if size := len(s.backlog); check != 252 && count != 4 && size == 7 {
		t.Fatalf("Expected Count: 4, got: %d, Expected: int 252, got: %d, expected size: 7, got: %d", count, check, size)
	}
	s.buffer = buff[15:16]
	s.byteReader(1, onInt)
	if size := len(s.backlog); check != 507 && count != 5 && size == 0 {
		t.Fatalf("Expected Count: 5, got: %d, Expected: int 507 , got: %d, expected size: 0, got: %d", count, check, size)
	}

}

func TestJobId(t *testing.T) {

	a := NextJobId()
	b := NextJobId()
	if a == b {
		t.Fatalf("exepected a!=b")
	}
	t.Logf("New job id: %d", NextJobId())
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

	// Synthetic Job
	ph, _, f := createRJob(nil)
	defer f.Close()
	job, _ := ph.(*CallBackJob)
	job.Events = 0
	w.throttle <- struct{}{}
	w.que <- job
	w.write.Write([]byte{1})
	w.ctrl.CheckTimeOut(0, 0)

	// should not throw errors
	e = w.Stop()
	w.ctrl.ProcessEvents(0, 0)
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)

	go func() {
		w.SingleRun()
		cancel()
	}()
	<-ctx.Done()

	e = w.Stop()
	if e != ERR_SHUTDOWN {
		t.Fatalf("Should have gotten a shutdown error, got: %v", e)
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
	_, _, e = w.ctrl.ProcessEvents(0, 0)
	if e != ERR_SHUTDOWN {
		t.Fatalf("error should be in shutdown mode!")
	}

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
		s.singleLoop()
		t.Logf("Job Count: %d", s.JobCount())
		t.Log("poll test completed")
	}()

	<-ctx.Done()

	if s.JobCount() != 1 {

		t.Fatalf("Job count should be 1.. if not something went wrong!\n")

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
	w.Job.OnEventCallBack = func(c *CallbackEvent) {
		e = c.Error()
	}
	w.singleLoop(t)
	if e == nil {
		t.Fatalf("Did not get our EOF event?")
	}
}

func TestWorkerJobRead(t *testing.T) {
	w := spawnRJobAndWorker(t)
	c := ""
	b := make([]byte, 2)

	w.Job.SetTimeout(50)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer w.WorkerCleanup()
	defer cancel()
	stop := false
	w.Job.OnEventCallBack = func(config *CallbackEvent) {
		if config.InError() {
			stop = true
			t.Logf("Got the following error: %v", config.error)
			stop = true
			return
		}
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
		t.Logf("Timeout is: %d", w.Job.Timeout)
		if x != nil {
			panic(x)
		}
		c += string(b[:s])
		if c == TEST_STRING {
			w.w.Close()
			t.Log("Got full string and closed writer")
		}
	}

	fail := true
	go func() {
		// full test
		for !stop {
			w.Worker.SingleRun()
		}
		t.Logf("Entering shutdown")
		w.Worker.Stop()
		w.Worker.SingleRun()
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
	w.Job.SetTimeout(2)
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer w.WorkerCleanup()
	defer cancel()
	var timeout bool
	var diff int64 = 0
	now := time.Now().UnixMilli()
	var e error
	w.Job.OnEventCallBack = func(config *CallbackEvent) {
		if timeout = config.InTimeout(); timeout {
			e = config.Error()
			diff = time.Now().UnixMilli() - now
			t.Log("Timeout Check completed")
			t.Logf("Throttle que size is: %d", len(w.Worker.throttle))
			t.Log("Error Check Completed, Shutting the worker down")
			w.Worker.Stop()
		}
	}
	go func() {
		t.Logf("Throttle que size is: %d", len(w.Worker.throttle))
		w.Worker.Start()
		defer cancel()
	}()
	<-ctx.Done()

	t.Logf("Our diff was: %d", diff)
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

	job.SetTimeout(2)
	defer worker.Stop()
	defer w.Close()
	var timeout bool
	var diff int64 = 0
	now := time.Now().UnixMilli()
	var e error
	ctx, cancel = context.WithTimeout(context.Background(), 2*time.Second)
	job.OnEventCallBack = func(config *CallbackEvent) {
		if timeout = config.InTimeout(); timeout {
			diff = time.Now().UnixMilli() - now
			t.Log("Timeout Check completed")
			t.Log("Error Check Completed, Shutting the worker down")
			e = config.Error()
			worker.Stop()
			t.Log("got to shutdown")
		}
	}
	worker.AddJob(job)
	go func() {
		worker.Start()
		defer cancel()
	}()
	<-ctx.Done()

	t.Logf("Our diff was: %d", diff)
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
	w.Job.SetTimeout(1)
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
		jobs[i].OnEventCallBack = func(e *CallbackEvent) {
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
			t.Fatalf("Job: %d, did not complete", i)
			return
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
		Timeout:  2,
		RawJobId: NextJobId(),
		OnEventCallBack: func(c *CallbackEvent) {
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
	w.Job.OnEventCallBack = func(config *CallbackEvent) {
		t.Logf("In Callback")

		if !config.InTimeout() {
			t.Logf("Something went wrong")
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
			w.Worker.SingleRun()
		}
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

	w.Job.OnEventCallBack = func(config *CallbackEvent) {
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
		t.Logf("On Callbaack pass: %d", count)
		switch count {
		case 0:
			config.PollRead()
			t.Log("Writing first Chunk [Hello ]")
			w.Write([]byte("Hello "))
			if !config.IsWrite() {
				t.Fatalf("Should be in write mode")
			}
			config.SetTimeout(1)
		case 1:
			if ok := config.InTimeout(); !ok {
				t.Fatalf("Failed to get our timeout")
			} else {
				t.Logf("Got timeout notice!")
			}
			t.Log("Swapping back to write poll")
			config.PollWrite()
			config.SetTimeout(0)
		case 2:
			t.Log("Writing second Chunk [Word!]")
			if !config.IsWrite() {
				t.Fatalf("Should be in write mode")
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
		w.Worker.SingleRun()
		// force the job to have a wokrer before we start
		w.Job.SetEvents(CAN_END)
		for w.Worker.JobCount() != 0 {
			w.Worker.SingleRun()
		}
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
		e := w.AddJob(&CallBackJob{RawJobId: NextJobId()})
		if e != nil {
			t.Fatalf("Should be able to add as many jobs as we like!")
		}
	}
	//w.SingleRun()
	w.singleLoop()
	t.Log(w.PoolUsageString())
	w.Stop()
	// if somehting is not worek
	w.Start()
	w.Start()
	if !w.closed {
		t.Fatalf("We should now be closed!")
	}

}

func TestUtilTimeout(t *testing.T) {
	w, _ := NewLocalWorker(0)
	defer w.Stop()
	u := w.NewUtil()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*2)
	count := 0
	_, e := u.SetTimeout(func(_ *CallbackEvent) { count++ }, 1)
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
	code := -1
	closed := false
	_, err := u.WaitPid(cmd.Process.Pid, func(e *WaitPidEvent) {
		// will not wait becase we know the process has exited all ready!
		closed = true
		code = e.ExitCode
		cmd.Process.Release()
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
	job.Release()
	job.Release()

	w.SingleRun()
	if closed {
		t.Fatalf("Expected this to be open")
	}

	cmd.Process.Kill()
	w.SingleRun()

	if code != 137 {
		t.Fatalf("Expected exit code to be 137, got: %d", code)
	}
	if !closed {
		t.Fatalf("Expected this to be closed")
	}
	if noRun {
		t.Fatalf("Watcher was not propery released!")
	}

	cmd2 := exec.Command("bash", "-c", "sleep 0.01;exit 2")
	e = cmd2.Start()
	if e != nil {
		t.Skipf("ms sleep failed??")
		return
	}
	defer cmd2.Process.Kill()

	closed = false
	job, err = u.WaitPid(cmd2.Process.Pid,
		func(e *WaitPidEvent) {
			code = e.ExitCode
			closed = true
			cmd2.Process.Release()
		},
	)
	if err != nil {
		t.Skipf("Could not watch our child??")
	}
	// load the job
	w.SingleRun()
	w.SingleRun()
	if !closed {
		t.Fatalf("Process did not exit")
	}
	t.Logf("Got exit code: %d", code)
	if code != 2 {
		t.Fatalf("Expected exit code to be 2, got: %d", code)
	}

}

func TestWritePollReader(t *testing.T) {
	p := createLocalWorker()
	canWrite := false
	didTo := false
	j, _, w := createRJob(func(config *CallbackEvent) {
		didTo = config.InTimeout()
		canWrite = config.IsWrite()
	})
	job, _ := j.(*CallBackJob)
	job.SetTimeout(1)
	job.SetEvents(CAN_RW)
	defer w.Close()
	defer p.Stop()
	p.AddJob(job)
	p.singleLoop()

	t.Logf("CanWrite: %v", canWrite)
	t.Logf("Was Timout: %v", didTo)
	p.Stop()
	p.SingleRun()
	if !p.closed {
		t.Fatalf("Failed, should be in a closed state")
	}

}

func TestValdiateJobClearOnTimeStampChange(t *testing.T) {
	p, _ := NewLocalWorker(2)

	var r *os.File
	var w *os.File
	res := make([]byte, 0)
	x, r, w := createRJob(func(config *CallbackEvent) {
		if config.IsRead() {
			buff := make([]byte, 1024)
			n, _ := r.Read(buff)
			res = append(res, buff[0:n]...)
		}
	})
	j, _ := x.(*CallBackJob)
	j.SetTimeout(50)
	p.AddJob(j)
	// force our job to be picked up
	p.SingleRun()

	w.Write([]byte("Hello,"))

	defer func() {
		w.Close()
		p.Stop()
		p.SingleRun()
	}()
	p.SingleRun()
	if len(res) == 0 {
		t.Fatalf("Expected the data to be read")
	}
	u := p.NewUtil()

	ran := false

	t.Logf("Adding timeout job")
	job, e := u.SetTimeout(func(_ *CallbackEvent) {
		t.Logf("Timer woke up")
		w.Write([]byte(" World!"))
		ran = true
	}, 5)
	if e != nil {
		t.Logf("Failed to create our job, error was: %v", e)
		t.Skip()
		return
	}
	t.Logf("Running load")
	// Force our object to load
	p.SingleRun()
	// force our job to be picked up
	t.Logf("Starting manual timer run")
	now := time.Now()
	p.now = time.UnixMilli(now.UnixMilli() + job.Timeout + 1)
	p.processNextSet(0)
	t.Logf("Finished")

	p.SingleRun()
	if !ran {
		t.Fatalf("Failed to run our timeout")
	}
	// one more pass to read from the buffer

	if "Hello, World!" != string(res) {
		t.Fatalf("Failed to get our string")
	}
	//p.Stop()
	//p.SingleRun()

}

func TestUpdateTimeoutOnly(t *testing.T) {
	p, e := NewLocalWorker(1)
	if e != nil {
		t.Skip()
	}
	defer func() {
		p.Stop()
		p.SingleRun()
	}()
	u := p.NewUtil()

	count := 0
	var job *CallBackJob
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	ctx2, cancel2 := context.WithTimeout(context.Background(), 1*time.Second)
	done := make(chan any, 1)
	job, e = u.SetInterval(func(event *CallbackEvent) {
		count++
		e = event.error
		t.Logf("On Count; %d, error was: %v", count, e)
		if count == 2 {
			done <- nil
			cancel()
		}
	}, 3000)
	if e != nil {
		t.SkipNow()
	}

	// load the job
	p.SingleRun()
	var timeout int64
	var ok bool
	if timeout, ok = p.timeouts.FirstKey(); !ok {
		t.Fatalf("Failed to load our job?")
	}
	job.SetTimeout(2)
	// reconfigure the job
	t.Logf("Reloading job config")
	p.SingleRun()
	// run the job

	if new_ts, ok := p.timeouts.FirstKey(); !ok || timeout == new_ts {
		t.Fatalf("Did not cofnugre out job, new ts: %d, old ts: %d", new_ts, timeout)
	} else {
		timeout = new_ts
	}
	if p.timeouts.Size() != 1 {
		t.Fatalf("Should only have one job, have %d", p.timeouts.Size())
	}

	t.Logf("Job should Run here")
	go func() {
		for {
			select {
			case <-done:
				cancel2()
				return
			default:
				p.SingleRun()
			}
		}
	}()
	<-ctx.Done()

	// Run the updated job
	t.Logf("Context window completed")

	if count != 2 {
		diff := time.Now().UnixMilli() - timeout

		t.Fatalf("2nd pass did not run?, timeout was: %d, time diff: %d, JobId: %d", job.Timeout, diff, job.JobId())
		return
	}

	<-ctx2.Done()
	t.Logf("Entering shutdown phaze")
	// shut down
	p.Stop()
	p.SingleRun()
	if e == nil {
		t.Fatalf("Never got shutdown notice?")
	}
	t.Logf("Error was: %v", e)

	if count != 3 && e != nil {
		t.Fatalf("Expected job to run 3 times, ran: %d", count)
	}
}

func TestUnixCron(t *testing.T) {

	cron := "17 * * * *"
	p, _ := NewLocalWorker(1)
	defer func() {
		p.Stop()
		p.SingleRun()
	}()
	u := p.NewUtil()
	count := 0
	var e error
	job, e := u.SetCron(func(event *CallbackEvent) {
		e = event.error
		t.Logf("Callback ran, error was: %v", e)
		count++
	}, cron)
	if e != nil {
		t.Fatalf("Should have set this cronjob up!")
	}
	// load our job
	p.SingleRun()
	now := time.Now()
	// the +1 is due to a semi random rounding error
	p.now = time.UnixMilli(now.UnixMilli() + job.Timeout + 1)

	p.processNextSet(0)

	if p.timeouts.Size() != 1 {
		t.Fatalf("job did not re-schedule itself???")
	}
	t.Logf("Current time is:           %s", time.Now().String())
	t.Logf("Using future timestamp of: %s", p.now.String())

	if count != 1 && e != nil {
		t.Fatalf("Expected a single run, got, %d and error of: %v", count, e)
	}
	p.Stop()
	p.SingleRun()

	if e == nil || count != 2 {
		t.Fatalf("Should have gotten a shutdown error: %v, count: %d", e, count)
	}

	_, e = u.SetCron(func(event *CallbackEvent) {}, "Bad Cron job")
	if e == nil {
		t.Fatalf("Should fail to parse a bad cron string!")
	}
}

func TestOpen2(t *testing.T) {

	p, _ := NewLocalWorker(2)
	u := p.NewUtil()
	defer p.Stop()
	defer p.singleLoop()
	code := 181 // Bull Shannon IQ!
	buff := make([]byte, 12)
	size := 0
	pcount := 0
	job, stdin, stdout, e := u.Open2(
		func(wpe *WaitPidEvent) {
			pcount++
			code = wpe.ExitCode
			t.Logf("Wait pid got called: %d, exit code: %d", pcount, code)

		},
		"/usr/bin/echo", "\"0.001\"",
	)
	if e != nil {
		t.Fatalf("Failed to Spawn Job, error was: %v", e)
		return
	}
	defer stdin.Close()
	defer stdout.Close()

	count := 0
	_, e = u.WatchRead(func(ce *CallbackEvent) {
		count++
		t.Logf("Getting called: %d", count)
		if ce.InError() {
			e = ce.error
			t.Logf("Error Polling stdout: %v", e)
		} else {

			size, e = stdout.Read(buff)
			t.Logf("Got chunk: %s", string(buff))
		}
		ce.Release()
	}, stdout, 2000)
	if e != nil {
		t.Fatalf("Unknown issue setting up reader: %v", e)
		return
	}

	t.Logf("Working with pid: %d", job.Pid)
	// load our jobs, we have 2
	//p.SingleRun()
	p.singleLoop()
	if p.JobCount() != 2 {
		t.Fatalf("Something went wrong?, epected 2 jobs, got %d", p.JobCount())
	}

	t.Logf("doing Single run")
	for pass := range 4 {
		// should only need 2 passes, but we may need more
		t.Logf("On pass:%d", pass)
		p.SingleRun()
		if e != nil {
			t.Fatalf("Got an error")
			break
		}
		if p.JobCount() == 0 {
			break
		}
		t.Logf("We have a total of: %d jobs remaining", p.JobCount())
	}
	if code != 0 || size == 0 {
		t.Fatalf("Expected code: 0, got: %d, expected size: !=0, got 0", code)
	}
}

func TestOpen3(t *testing.T) {

	p, _ := NewLocalWorker(0)
	u := p.NewUtil()
	defer p.Stop()
	defer p.singleLoop()
	code := 181 // Bull Shannon IQ!
	buff := make([]byte, 55)
	size := 0
	pcount := 0
	job, stdin, stdout, stderr, e := u.Open3(
		func(wpe *WaitPidEvent) {
			pcount++
			code = wpe.ExitCode
			t.Logf("**** Wait pid got called: %d, exit code: %d", pcount, code)

		},
		"bash", "-c", "cat;echo 'force extra read';echo 'stdin was closed!'>&2;exit 64",
	)
	if e != nil {
		t.Fatalf("Failed to Spawn Job, error was: %v", e)
		return
	}
	defer stdin.Close()
	defer stdout.Close()
	defer stderr.Close()

	count := 0
	_, e = u.WatchRead(func(ce *CallbackEvent) {
		count++
		t.Logf("Stdout read pass: %d", count)
		if ce.InError() {
			if count < 3 {
				e = ce.error
			}
			t.Logf("Error Polling stdout: %v", ce.Error())

		} else {

			size, e = stdout.Read(buff)
			t.Logf("Stdout Got chunk: %s", string(buff[0:size]))
		}
	}, stdout, 2000)
	if e != nil {
		t.Fatalf("Unknown issue setting up reader: %v", e)
		return
	}

	ecount := 0

	_, e = u.WatchRead(func(ce *CallbackEvent) {
		ecount++
		if ce.InError() {
			e = ce.error
			t.Logf("Error Polling stdout: %v", e)
		} else {

			size, e = stderr.Read(buff)
			t.Logf("Stderr Got chunk: %s", string(buff[0:size]))
		}
		ce.Release()
	}, stderr, 2000)
	if e != nil {
		t.Fatalf("Unknown issue setting up reader: %v", e)
		return
	}

	t.Logf("Working with pid: %d", job.Pid)
	// load our jobs, we have 2
	//p.SingleRun()
	p.singleLoop()
	if p.JobCount() != 3 {
		t.Fatalf("Something went wrong?, epected 3 jobs, got %d", p.JobCount())
	}

	stdin.Write([]byte("Hello, "))
	t.Logf("doing Single run")
	for pass := range 10 {
		// should only need 2 passes, but we may need more
		t.Logf("On pass:%d", pass)
		//p.SingleRun()
		p.SingleRun()
		if e != nil {
			//t.Fatalf("Got an error, %v", e)
			break
		}
		switch count {
		case 1:
			t.Logf("Writing 2nd chunl")
			stdin.Write([]byte("World!\n"))
		case 2:
			t.Logf("Closing stdin")
			stdin.Close()
		}
		if p.JobCount() == 0 {
			t.Logf("All Jobs completed!")
			break
		}
		t.Logf("We have a total of: %d jobs remaining", p.JobCount())
	}

	if code != 64 || size == 0 || ecount == 0 {
		t.Fatalf("Expected code: 64, got: %d, expected size: !=0, got 0", code)
	}
}

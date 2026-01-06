package osfp

import (
	"os"
	"os/signal"
	"syscall"
	"unsafe"

	"golang.org/x/sys/unix"
)

type Util struct {
	PoolOrWorkerContainer
}

// Creates a timout that runs once executing the cb function provided. The timeout valie is in milliseconds. You can terminate the timeout,
// by calling the *CallBackJob.Release() method.
func (s *Util) SetTimeout(cb func(), timeout int64) (*CallBackJob, error) {
	job := &CallBackJob{
		onEvent: func(event *CallbackEvent) { cb() },
		timeout: timeout,
	}

	return job, s.AddJob(job)
}

// Creates an timer that will continue to run at regular intervals until terminatedd.  The interval value is in milliseconds.  To terminate the
// can either calling the *CallBackJob.Release() method or by calling the *CallbackEvent.Release() method.
func (s *Util) SetInterval(cb func(event *CallbackEvent), interval int64) (*CallBackJob, error) {
	job := &CallBackJob{
		onEvent: func(event *CallbackEvent) {
			if event.InTimeout() {
				event.SetTimeout(interval)
			}
			cb(event)
		},
		timeout: interval,
	}

	return job, s.AddJob(job)
}

// Creates a signal listener job for the pool.
//
// # Notes on Signals
//
// This uses signal.Ignore(...) for every value in signals.  If you reqire restoring the default signal handlerss after this operation,
// you will need to call signal.Reset(...).
//
// # NOTES On Testing and go run
//
// Calling this operation from : "go run myapp.go" or "go test" will most likely result in the /proc/processid/status, SigBlk field: being
// set to 0. This is a quirk of the golang build an testing frameworks.  To test this integration properly you may have
// to compile and run your code directly to verify signal handlers are applied properly.
func (s *Util) SigWatcher(signals []syscall.Signal, cb func(event *SigEvent)) (*SigWatchJob, error) {
	sigset := &unix.Sigset_t{}
	for _, sig := range signals {
		// Signal numbers are 1-indexed. Bit 0 corresponds to signal 1.
		if sig <= 0 {
			continue
		}
		// force the go runtime to ignore our signal
		signal.Ignore(sig)
		sigNum := uint(sig) - 1
		pos := sigNum / 64

		var shift uint64 = 1 << (sigNum % 64)
		sigset.Val[pos] |= shift
	}

	// Block the current process and threads from getting these signals!
	if err := unix.PthreadSigmask(unix.SIG_BLOCK, sigset, nil); err != nil {
		return nil, err
	}

	var info unix.SignalfdSiginfo
	size := unsafe.Sizeof(info)
	buf := make([]byte, size)
	sfd, err := unix.Signalfd(-1, sigset, 0)
	if err != nil {
		unix.Close(sfd)
		return nil, err
	}

	// we use os.File becuase of the .19mb read bug in unix.Read package
	file := os.NewFile(uintptr(sfd), "Signal Pipe")
	wrap := func(event *CallbackEvent) {
		sige := &SigEvent{
			CallbackEvent: event,
			File:          file,
		}
		if event.InError() {
			cb(sige)
			return
		}

		n, err := file.Read(buf)
		if err != nil {
			file.Close()
			event.Release()
			event.error = err
			cb(sige)
			return
		}
		if n != int(size) {
			// not a complete write by the os yet!
			return
		}
		// Directly cast the buffer to the struct pointer
		// This is faster and avoids needing to check endianness
		info = *(*unix.SignalfdSiginfo)(unsafe.Pointer(&buf[0]))

		for _, sig := range signals {
			if info.Signo == uint32(sig) {
				sige.Signal = sig
				cb(sige)
				return
			}
		}
	}
	job := &SigWatchJob{
		File: file,
		CallBackJob: &CallBackJob{
			onEvent: wrap,
			fd:      int32(sfd),
			events:  CAN_READ,
		},
	}

	s.AddJob(job)

	return job, nil
}

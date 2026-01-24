package osfp

// low level tls stuffs
//  https://stackoverflow.com/questions/71366504/low-level-tls-handshake
import (
	"fmt"
	"os"
	"time"

	"github.com/akalinux/os-file-pooler/internal/sec"
	Cmd "github.com/akalinux/os-file-pooler/pkg/Cmd"
	"github.com/aptible/supercronic/cronexpr"
	"golang.org/x/sys/unix"
)

type Util struct {
	PoolOrWorkerContainer
}

// Creates a timer that runs once executing the cb function provided. The timeout value is in milliseconds. You can terminate the timeout,
// by calling the *CallBackJob.Release() method.
func (s *Util) SetTimeout(cb func(event *CallbackEvent), timeout int64) (*CallBackJob, error) {
	job := &CallBackJob{
		FdId: -1,
		OnEventCallBack: func(event *CallbackEvent) {
			cb(event)
		},
		Timeout: timeout,
	}

	return job, s.AddJob(job)
}

// Creates an timer that will continue to run at regular intervals until terminatedd.  The interval value is in milliseconds.  To terminate the
// can either calling the *CallBackJob.Release() method or by calling the *CallbackEvent.Release() method.
func (s *Util) SetInterval(cb func(event *CallbackEvent), interval int64) (*CallBackJob, error) {

	job := &CallBackJob{
		FdId: -1,
		OnEventCallBack: func(event *CallbackEvent) {
			if event.InTimeout() {
				event.SetTimeout(event.timeout)
			}
			cb(event)
		},
		Timeout: interval,
	}

	return job, s.AddJob(job)
}

func (s *Util) Open2(cb func(*WaitPidEvent), name string, args ...string) (job *CmdJob, stdin *os.File, stdout *os.File, err error) {
	return s.open(func(c *Cmd.Cmd) error { return nil }, cb, name, args...)
}
func (s *Util) open(bfeoreStart func(*Cmd.Cmd) error, cb func(*WaitPidEvent), name string, args ...string) (job *CmdJob, stdin *os.File, stdout *os.File, err error) {

	cmd := Cmd.NewCmd(name, args...)
	if stdin, err = cmd.NewStdin(); err != nil {
		cmd.CloseFd()
		return
	}

	if stdout, err = cmd.NewStdout(); err != nil {
		cmd.CloseFd()
		return
	}

	if err = bfeoreStart(cmd); err != nil {
		cmd.CloseFd()
		return
	}
	var proc *os.Process
	if proc, err = cmd.Start(); err != nil {
		cmd.CloseFd()
		return
	}
	wp, err := s.WaitPid(proc.Pid, func(wpe *WaitPidEvent) {
		proc.Release()
		wpe.Release()
		cb(wpe)
	})
	if err != nil {
		cmd.CloseFd()
		proc.Kill()
		return
	}
	job = &CmdJob{
		Process:    proc,
		WaitPidJob: wp,
	}

	return
}

func (s *Util) Open3(cb func(*WaitPidEvent), name string, args ...string) (job *CmdJob, stdin *os.File, stdout *os.File, stderr *os.File, err error) {

	job, stdin, stdout, err = s.open(func(c *Cmd.Cmd) error {
		stderr, err = c.NewStderr()
		return err
	}, cb, name, args...)

	return
}

// Watches a given pid and runs cb on exit.  The process exit code can be found in *WaitPidEvent.ExitCode.
func (s *Util) WaitPid(pid int, cb func(*WaitPidEvent)) (*WaitPidJob, error) {
	pfd, err := unix.PidfdOpen(pid, unix.PIDFD_NONBLOCK)

	if err != nil {
		// no such pid
		return nil, fmt.Errorf("Failed to Create fd for pid: %d, erro was %w", pid, err)
	}

	var job *WaitPidJob
	job = &WaitPidJob{
		pid: pid,
		fd:  &pfd,
		CallBackJob: &CallBackJob{
			FdId:   int32(pfd),
			Events: CAN_READ,
			OnEventCallBack: func(event *CallbackEvent) {

				if pfd == -1 {
					// we have been closed!
					return
				}
				pe := &WaitPidEvent{
					CallbackEvent: event,
					Usage:         &unix.Rusage{},
					Info:          &unix.Siginfo{},
				}
				defer job.closeFd()
				defer cb(pe)
				if event.InError() {
					return
				}
				event.error = unix.Waitid(unix.P_PIDFD, pfd, pe.Info, unix.WNOHANG|unix.WEXITED, pe.Usage)
				pe.ExitCode = sec.GetExitCodeFromSigInfo(pe.Info)

			},
		},
	}
	return job, s.AddJob(job)
}

func (s *Util) SetCron(cb func(event *CallbackEvent), cron string) (*CallBackJob, error) {

	expr, err := cronexpr.Parse(cron)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	next := expr.Next(now)
	interval := next.UnixMilli() - now.UnixMilli()
	job := &CallBackJob{
		FdId: -1,
		OnEventCallBack: func(event *CallbackEvent) {
			now := event.GetNow()
			next := expr.Next(now)
			interval := next.UnixMilli() - now.UnixMilli()
			if event.InTimeout() {
				event.SetTimeout(interval)
			}
			cb(event)
		},
		Timeout: interval,
	}

	return job, s.AddJob(job)
}

func (s *Util) WatchRead(cb func(*CallbackEvent), file *os.File, msTimeout int64) (job Job, err error) {
	job = &CallBackJob{
		Timeout:         msTimeout,
		Events:          CAN_READ,
		OnEventCallBack: cb,
		FdId:            int32(file.Fd()),
	}
	return job, s.AddJob(job)
}

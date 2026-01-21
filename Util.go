package osfp

import (
	"fmt"
	"time"

	"github.com/akalinux/os-file-pooler/internal/sec"
	"github.com/aptible/supercronic/cronexpr"
	"golang.org/x/sys/unix"
)

type Util struct {
	PoolOrWorkerContainer
}

// Creates a timout that runs once executing the cb function provided. The timeout value is in milliseconds. You can terminate the timeout,
// by calling the *CallBackJob.Release() method.
func (s *Util) SetTimeout(cb func(), timeout int64) (*CallBackJob, error) {
	job := &CallBackJob{
		FdId: -1,
		OnEventCallBack: func(event *CallbackEvent) {
			if event.InTimeout() {
				cb()
			}
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
				event.SetTimeout(interval)
				cb(event)
			}
		},
		Timeout: interval,
	}

	return job, s.AddJob(job)
}

// Watches a given pid and runs cb on exit.  The process exit code can be found in *WaitPidEvent.ExitCode.
func (s *Util) WaitPid(pid int, cb func(*WaitPidEvent)) (Job, error) {
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

func (s *Util) SetCron(cb func(), cron string) (*CallBackJob, error) {

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
			now = event.GetNow()
			next = expr.Next(now)
			interval = next.UnixMilli() - now.UnixMilli()
			if event.InTimeout() {
				event.SetTimeout(interval)
				cb()
			}
		},
		Timeout: interval,
	}

	return job, s.AddJob(job)
}

//go:build linux

package osfp

import (
	"fmt"

	"github.com/akalinux/os-file-pooler/internal/sec"
	"golang.org/x/sys/unix"
)

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

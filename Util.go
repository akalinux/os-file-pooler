package osfp

// low level tls stuffs
//  https://stackoverflow.com/questions/71366504/low-level-tls-handshake
import (
	"crypto/tls"
	"fmt"
	"net"
	"os"
	"reflect"
	"syscall"
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

// This method creates spawns the command with stdin, and stdout set as pipes, the callback will be run when the spawned process exists.
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

// This method creates spawns the command with stdin, stdout, and stderr set as pipes, the callback will be run when the spawned process exists.
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

// Spawns a job that runs the given callback at the set cron interval.  This callback runs in the shared event loop.
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

// Tries to find the underlying file descriptor for a given net.Conn interface instance.
// Works for tls/nontls/unix/tcp/udp
func ConnToFd(src net.Conn) (fd int32, err error) {

	var conn = src
	if tlsConn, ok := conn.(*tls.Conn); ok {
		// convert from tls to normal net.Conn
		conn = tlsConn.NetConn()
	}
	if sc, ok := conn.(syscall.Conn); ok {
		rawConn, err := sc.SyscallConn()

		if err != nil {
			return 0, err
		}
		err = rawConn.Control(func(sysFD uintptr) {
			// sysFD is the *actual* file descriptor, no dup involved.
			fd = int32(sysFD)
		})
		if err != nil {
			return 0, err
		}
		return fd, nil
	} else {
		// try fallback in case the above fails
		func() {
			defer func() {
				if r := recover(); r != nil {
					err = fmt.Errorf("could not find fd via reflect")
				}
			}()
		}()
		tcpConn := reflect.Indirect(reflect.ValueOf(conn)).FieldByName("conn")
		fdVal := tcpConn.FieldByName("fd")
		pfdVal := reflect.Indirect(fdVal).FieldByName("pfd")

		fd = int32(pfdVal.FieldByName("Sysfd").Int())
		return
	}

}

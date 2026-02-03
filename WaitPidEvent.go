package osfp

import "golang.org/x/sys/unix"

type WaitPidEvent struct {
	Info  *unix.Siginfo
	Usage *unix.Rusage
	AsyncEvent
	ExitCode int
}

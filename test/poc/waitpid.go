package main

import (
	"fmt"
	"os"
	"runtime"

	"golang.org/x/sys/unix"
)

func watchPidAsync(pid int) (<-chan struct{}, error) {
	// 1. Obtain a pidfd for the target PID
	// PIDFD_NONBLOCK (available since Linux 5.10) ensures we don't block
	pfd, err := unix.PidfdOpen(pid, unix.PIDFD_NONBLOCK)
	if err != nil {
		return nil, fmt.Errorf("pidfd_open error: %w", err)
	}

	// 2. Convert raw FD to os.File to integrate with Go's Netpoller
	// This makes it compatible with non-blocking I/O
	f := os.NewFile(uintptr(pfd), fmt.Sprintf("pidfd-%d", pid))
	done := make(chan struct{})

	go func() {
		defer f.Close()
		defer close(done)

		// 3. This Read call will "block" the goroutine, but NOT the OS thread.
		// Go's runtime will wake this goroutine only when the process exits.
		// For pidfds, an EPOLLIN event indicates process termination.
		buf := make([]byte, 1)
		_, _ = f.Read(buf)
		fmt.Printf("Process %d has terminated.\n", pid)
	}()

	return done, nil
}

func main() {
	if runtime.GOOS != "linux" {
		panic("pidfd is only available on Linux")
	}

	pid := 1234 // Example PID
	exitChan, err := watchPidAsync(pid)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	fmt.Println("Watching process... Main thread is free.")
	<-exitChan
	fmt.Println("Cleanup complete.")
}

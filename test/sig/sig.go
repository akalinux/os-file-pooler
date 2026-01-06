package main

import (
	"fmt"
	"log"
	"os"

	//"runtime"
	"unsafe"

	"os/signal"
	"syscall"

	"golang.org/x/sys/unix"
)

func main() {
	//runtime.LockOSThread()
	signals := []syscall.Signal{unix.SIGHUP, unix.SIGINT}
	sigset := &unix.Sigset_t{}
	for _, sig := range signals {
		// Signal numbers are 1-indexed. Bit 0 corresponds to signal 1.
		if sig <= 0 {
			continue
		}
		sigNum := uint(sig) - 1
		pos := sigNum / 64

		var shift uint64 = 1 << (sigNum % 64)
		fmt.Printf("Sig: %d pos: %d,shift: %d\n", sigNum, pos, shift)
		sigset.Val[pos] |= shift
		fmt.Printf("Sig: %d pos: %d,shift: %d resolved %d\n", sigNum, pos, shift, sigset.Val[pos])
		signal.Ignore(sig)
	}

	if err := unix.PthreadSigmask(unix.SIG_BLOCK, sigset, nil); err != nil {
		panic("Could not apply sig mask")
	}

	// Create signalfd
	sfd, err := unix.Signalfd(-1, sigset, 0)
	if err != nil {
		log.Fatal(err)
	}
	defer unix.Close(sfd)

	fmt.Printf("Our pid is: %d\nWaiting for signal...", os.Getppid())
	r := os.NewFile(uintptr(sfd), "pid pipe")

	// Prepare buffer sized exactly to the struct
	var info unix.SignalfdSiginfo
	size := unsafe.Sizeof(info)
	buf := make([]byte, size)

	for {
		//n, err := unix.Read(sfd, buf)
		n, err := r.Read(buf)
		if err != nil {
			log.Fatal(err)
		}
		if n != int(size) {
			continue
		}

		// Directly cast the buffer to the struct pointer
		// This is faster and avoids needing to check endianness
		info = *(*unix.SignalfdSiginfo)(unsafe.Pointer(&buf[0]))
		fmt.Printf("Size of size: %d\n", size)

		fmt.Printf("\nReceived signal: %d\n", int32(info.Signo))
		//os.Exit(0)
	}
}

package main

import (
	"fmt"
	"log"
	"os"
	"unsafe"

	"golang.org/x/sys/unix"
)

func addSignal(set *unix.Sigset_t, sig unix.Signal) {
	// Signal numbers are 1-indexed. Bit 0 corresponds to signal 1.
	if sig <= 0 {
		return
	}
	sigNum := uint(sig) - 1
	// On most 64-bit systems, Val is [16]uint64
	set.Val[sigNum/64] |= 1 << (sigNum % 64)
}

func main() {
	// Block signals using thread-safe PthreadSigmask
	sigset := &unix.Sigset_t{}
	// Manually add signals to the set
	//addSignal(sigset, unix.SIGINT)
	//addSignal(sigset, unix.SIGINT)
	addSignal(sigset, unix.SIGHUP)

	if err := unix.PthreadSigmask(unix.SIG_BLOCK, sigset, nil); err != nil {
		log.Fatal(err)
	}

	// Create signalfd
	sfd, err := unix.Signalfd(-1, sigset, 0)
	if err != nil {
		log.Fatal(err)
	}
	defer unix.Close(sfd)

	fmt.Println("Waiting for signal...")

	// Prepare buffer sized exactly to the struct
	var info unix.SignalfdSiginfo
	buf := make([]byte, unsafe.Sizeof(info))

	for {
		n, err := unix.Read(sfd, buf)
		if err != nil {
			log.Fatal(err)
		}
		if n != int(unsafe.Sizeof(info)) {
			continue
		}

		// Directly cast the buffer to the struct pointer
		// This is faster and avoids needing to check endianness
		info = *(*unix.SignalfdSiginfo)(unsafe.Pointer(&buf[0]))

		fmt.Printf("Received signal: %d\n", info.Signo)

		if info.Signo == uint32(unix.SIGINT) || info.Signo == uint32(unix.SIGTERM) {
			os.Exit(0)
		}
	}
}

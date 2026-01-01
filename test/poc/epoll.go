package main

// https://cs.opensource.google/go/x/sys/+/refs/tags/v0.39.0:unix/ztypes_linux.go;l=916

import (
	"fmt"
	"os"
	"time"

	"syscall"

	"golang.org/x/sys/unix"
)

const IN_ERROR = unix.POLLERR | // Errors
	unix.POLLHUP | // Other end has disconnected
	unix.POLLNVAL // Other end has closed
func main() {
	//err := unix.SetNonblock(int(os.Stdin.Fd()), true)
	//if err != nil {
	//	panic(err)
	//}
	r, w, e := os.Pipe()

	if e != nil {
		panic(e)
	}
	e = unix.SetNonblock(int(r.Fd()), true)
	if e != nil {
		panic(e)
	}

	fdin := unix.PollFd{
		Fd:     int32(r.Fd()),
		Events: unix.POLLIN | IN_ERROR,
	}
	count := 0
	readTotal := 0
	writeTotal := 0

	poll := &[]unix.PollFd{fdin}
	for {
		//n, e := unix.Poll(*poll, 0)
		n, e := unix.Poll(*poll, 0)
		if e != nil {
			fmt.Printf("Poll Error: %s\n", e)
			time.Sleep(1000000)
			continue
			//return
		}
		if n == 0 {
			count += 1
			fmt.Printf("No Read\n")
			//size, e := w.Write([]byte(fmt.Sprintf("%d", count)))
			size, e := syscall.Write(int(w.Fd()), []byte(fmt.Sprintf("%d", count)))
			if e != nil {

				fmt.Printf("Write Error: %s\n", e)
				fmt.Printf("Bytes written: %d, bytes read: %d\n", readTotal, writeTotal)
				return
			}
			writeTotal += size
			fmt.Printf("Wrote %d, sent: %d\n", count, size)
			continue
		}
		if fdin.Revents&IN_ERROR != 0 {
			fmt.Printf("We are done")
			return
		}
		buff := make([]byte, 2)
		n, e = syscall.Read(int(fdin.Fd), buff)
		if e != nil {
			fmt.Printf("Error Reading: %s\n", e)
			fmt.Printf("Bytes written: %d, bytes read: %d\n", readTotal, writeTotal)
			return
		}
		fmt.Printf("Got: [%s]\n", string(buff))
		readTotal += n
		fdin.Events = 0
	}

}

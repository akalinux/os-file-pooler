package main

// https://cs.opensource.google/go/x/sys/+/refs/tags/v0.39.0:unix/ztypes_linux.go;l=916

import (
	"fmt"
	"os"
	"time"

	"golang.org/x/sys/unix"
)

// https://man7.org/linux/man-pages/man2/poll.2.html
const IN_ERROR = unix.POLLERR | // Errors
	unix.POLLHUP | // Other end has disconnected
	unix.POLLNVAL | unix.POLLRDHUP // Other end has closed
func main() {
	//err := unix.SetNonblock(int(os.Stdin.Fd()), true)
	//if err != nil {
	//	panic(err)
	//}
	r, w, e := os.Pipe()

	if e != nil {
		panic(e)
	}
	e = unix.SetNonblock(int(w.Fd()), true)
	if e != nil {
		panic(e)
	}

	fdin := unix.PollFd{
		Fd:     int32(r.Fd()),
		Events: unix.POLLIN,
	}
	count := 0
	num := 0
	readTotal := 0
	writeTotal := 0

	poll := []unix.PollFd{fdin}
	buff := make([]byte, 1)
	for {
		//n, e := unix.Poll(*poll, 0)
		n, e := unix.Poll(poll, 0)
		if e != nil {
			fmt.Printf("Poll Error: %s\n", e)
			time.Sleep(500)
			continue
			//return
		}
		if n == 0 {

			count += 1
			num += 1000
			fmt.Printf("No Read\n")
			//size, e := w.Write([]byte(fmt.Sprintf("%d", count)))
			size, e := unix.Write(int(w.Fd()), []byte(fmt.Sprintf("%d", num)))
			if e != nil {

				fmt.Printf("Write Error: %s\n", e)
				fmt.Printf("Bytes written: %d, bytes read: %d\n", readTotal, writeTotal)
				return
			}
			writeTotal += size
			fmt.Printf("Wrote %d, sent: %d\n", count, size)
		} else {
			fdin := poll[0]
			if fdin.Revents == 0 {
				fmt.Printf("Got nothing\n")
			}
			if fdin.Revents&unix.POLLIN != 0 {
				buff = buff[:0]
				n, e = r.Read(buff[:cap(buff)])
				//n, e = unix.Read(int(fdin.Fd), buff[:cap(buff)])
				if e != nil {
					fmt.Printf("Error Reading: %s\n", e)
					fmt.Printf("Bytes written: %d, bytes read: %d\n", readTotal, writeTotal)
					return
				}
				fmt.Printf("Got: [%s]\n", string(buff[:n]))
				if fdin.Revents&IN_ERROR != 0 {
					fmt.Printf("Error detected durring read\n")
				}
				readTotal += n
				fdin.Revents = 0
			} else if fdin.Revents&IN_ERROR != 0 {
				fmt.Printf("Bytes written: %d, bytes read: %d\n", readTotal, writeTotal)
				switch {
				case fdin.Revents&unix.POLLERR != 0:
					fmt.Printf("Got an error\n")
				case fdin.Revents&unix.POLLHUP != 0:
					fmt.Printf("Got an error: Closed local hup\n")
				case fdin.Revents&unix.POLLRDHUP != 0:
					fmt.Printf("Got an error: Closed remote hup\n")
				case fdin.Revents&unix.POLLNVAL != 0:
					fmt.Printf("Got an error: Connection not valid\n")
				}
				fdin.Revents = 0
				fmt.Printf("We are done\n")
				return
			}

			if count > 9 {
				w.Close()
			}
		}
	}

}

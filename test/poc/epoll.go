package main

// https://cs.opensource.google/go/x/sys/+/refs/tags/v0.39.0:unix/ztypes_linux.go;l=916

import (
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

const IN_ERROR = unix.POLLERR | // Errors
	unix.POLLHUP | // Other end has disconnected
	unix.POLLNVAL // Other end has closed
func main() {
	fmt.Printf("%v\n", os.Stdin.Fd())
	fdin := unix.PollFd{
		Fd: int32(os.Stdin.Fd()),
		Events: unix.POLLIN | // Can Read
			IN_ERROR,
	}
	fout := unix.PollFd{
		Fd:      int32(os.Stdout.Fd()),
		Revents: 0,
		Events:  unix.POLLOUT | IN_ERROR, // Can Write
	}

	poll := &[]unix.PollFd{fdin}
	backlog := make([]byte, 0xffff)
	buff := make([]byte, 0xffff)

	/*
	* As a note, we can close this loop by adding a file handle that when closed shuts down the loop.
	* You can also write to that handle, when you do it signals new elements need to be added to the pool!
	 */
	for {
		fmt.Printf("Job size: %d\n", len(*poll))
		if len(*poll) == 0 {
			break
		}
		totalEvents, err := unix.Poll(*poll, 20)
		if err != nil {
			panic(err)
		}
		if totalEvents == 0 {
			continue
		}
		next := make([]unix.PollFd, 0, len(*poll))
		for _, h := range *poll {
			fmt.Printf("Got Event For: %d\n", h.Fd)
			if h.Fd == 0 {
				if h.Revents&unix.POLLIN != 0 {
					size, _ := unix.Read(int(h.Fd), buff)
					backlog = append(backlog, buff[:size]...)
					fmt.Printf("Got bytes: %d\n", size)
					buff = buff[:0]
					if IN_ERROR&h.Revents == 0 {

						next = append(next, h)
					} else {
						fmt.Println("Finished STDIN")
					}
				} else if IN_ERROR&h.Revents != 0 {
					fmt.Printf("STDIN IS DONE\n")
				}
			} else {
				// h.Fd==1 in this case!
				before := len(backlog)
				size, _ := unix.Write(int(h.Fd), backlog)
				backlog = backlog[size:]
				fmt.Printf("\nWrote: %d bytes, backlog is now: %d, backlog was %d\n", size, len(backlog), before)
			}
			h.Events = 0
		}
		if len(backlog) != 0 {
			fmt.Printf("polling stdout\n")
			next = append(next, fout)
		}
		if len(next) == 0 {
			fmt.Printf("No More work to do!")
			break
		}
		poll = &next

	}

}

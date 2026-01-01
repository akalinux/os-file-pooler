package main

import (
	"fmt"
	"io"
)

func main() {
	r, w := io.Pipe()

	go func() {
		count := 0
		for {
			count++
			w.Write(fmt.Appendf([]byte{}, "%d\n", count))
		}
	}()
	for {
		buff := make([]byte, 2)
		r.Read(buff)
		fmt.Printf("%s", buff)
	}
}

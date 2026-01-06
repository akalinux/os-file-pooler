package osfp

import (
	"os"

	"golang.org/x/sys/unix"
)

type SigEvent struct {
	*CallbackEvent
	Signal unix.Signal
	File   *os.File
}

package osfp

import (
	"os"
	//"golang.org/x/sys/unix"
)

type Worker struct {
	throttle chan any
	que      chan *Job
	file     *os.File
}

func NewWorker(que chan *Job, throttle chan any, file *os.File) *Worker {
	return &Worker{
		throttle,
		que,
		file,
	}
}

func (s *Worker) Run() {

}

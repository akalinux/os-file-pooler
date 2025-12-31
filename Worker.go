package osfp

import (
	"os"
	//"golang.org/x/sys/unix"
)

type Worker struct {
	throttle chan any
	que      chan *Job
	file     *os.File
	limit    int
}

func NewWorker(que chan *Job, throttle chan any, file *os.File, limit int) *Worker {
	return &Worker{
		throttle,
		que,
		file,
		limit,
	}
}

func (s *Worker) Run() {

}

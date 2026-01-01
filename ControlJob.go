package osfp

type ControlJob struct {
	worker *Worker
	que    chan Job
	buffer []byte
}

func NewControlJob(worker *Worker) *ControlJob {
	return &ControlJob{
		worker: worker,
		que:    make(chan Job),
		buffer: []byte{0},
	}
}

func (s *ControlJob) GetFlags() int {
	return CAN_READ
}

func (s *ControlJob) ProcessFlags(flags int16, now int64) (nextFlags int16, nextFutureTimeOut int64) {
	_, e := s.worker.file.Read(s.buffer)
	if e != nil {
		return 0, 0
	}
	return CAN_READ, 0
}

// notes that this job needs to shutdown.
func (s *ControlJob) Shutdown(error) {

}

func (s *ControlJob) SetPool(worker *Worker) (futureTimeout int64, fd int) {
	return 0, 0
}

func (s *ControlJob) ClearPool() {

}

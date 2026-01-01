package osfp

type ControlJob struct {
	fd  int
	que chan Job
}

func NewControlJob(pool *Worker) *ControlJob {
	return &ControlJob{
		fd:  int(pool.file.Fd()),
		que: make(chan Job),
	}
}

func (s *ControlJob) GetFlags() int {
	return HALT_POLLING | CAN_READ
}

func (s *ControlJob) ProcessFlags(flags int16, now int64) (nextFlags int16, nextFutureTimeOut int64) {
	if nextFlags&HALT_POLLING != 0 {

		return 0, 0
	}
	return 0, 0
}

// notes that this job needs to shutdown.
func (s *ControlJob) Shutdown(error) {

}

func (s *ControlJob) SetPool(worker *Worker) (futureTimeout int64, fd int) {
	return 0, 0
}

func (s *ControlJob) ClearPool()

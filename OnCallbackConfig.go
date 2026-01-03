package osfp

type OnCallBackConfig struct {
	// Update this value to set a new timeout
	Timeout int64
	// Update this value to change what Events we poll
	Events int16
}

// Sets the timeout for the listener in the current job.
func (s *OnCallBackConfig) SetTimeout(Timeout int64) {
	s.Timeout = Timeout
}

// Tells the Current worker to stop polling this job and remove it from the thread pool.
func (s *OnCallBackConfig) Release() {
	s.Events = CAN_END
}

// Tells current worker to poll reads for this job.
func (s *OnCallBackConfig) PollRead() {
	s.Events = CAN_READ
}

// Tells current worker poll writes for this job.
func (s *OnCallBackConfig) PollWrite() {
	s.Events = CAN_WRITE
}

// Tells current worker poll both read and write for this job.
func (s *OnCallBackConfig) PollReadWrite() {
	s.Events = CAN_RW
}

package osfp

type OnCallBackConfig struct {
	// Update this value to set a new timeout
	timeout int64
	// Update this value to change what events we poll
	events int16

	error error
}

// Returns the current error or nil
func (s *OnCallBackConfig) InError() error {
	return s.error
}

// Returns true if this this job is in a timeout.  If the job is in an error state error is not nil.
func (s *OnCallBackConfig) InTimeout() (bool, error) {
	return s.error == nil && s.timeout == 0 && s.events == 0, s.error
}

// Returns true if the job is ready to read.
func (s *OnCallBackConfig) IsRead() bool {
	return s.error == nil && s.events&CAN_READ != 0
}

// Returns true if the job is ready to write.
func (s *OnCallBackConfig) IsWrite() bool {
	return s.error == nil && s.events&CAN_WRITE != 0
}

// Returns true if the job ready to read and write.
func (s *OnCallBackConfig) IsRW() bool {
	return s.error == nil && s.events&CAN_READ != 0 && s.events&CAN_WRITE != 0
}

// Sets the timeout for the listener in the current job.
func (s *OnCallBackConfig) SetTimeout(Timeout int64) {
	s.timeout = Timeout
}

// Tells the Current worker to stop polling this job and remove it from the thread pool.
func (s *OnCallBackConfig) Release() {
	s.events = CAN_END
}

// Tells current worker to poll reads for this job.
func (s *OnCallBackConfig) PollRead() {
	s.events = CAN_READ
}

// Tells current worker poll writes for this job.
func (s *OnCallBackConfig) PollWrite() {
	s.events = CAN_WRITE
}

// Tells current worker poll both read and write for this job.
func (s *OnCallBackConfig) PollReadWrite() {
	s.events = CAN_RW
}

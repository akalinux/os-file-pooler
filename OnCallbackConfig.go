package osfp

import "os"

type OnCallBackConfig struct {
	timeout int64
	// Update this value to set a new timeout
	Timeout int64
	// Update this value to change what events we poll
	events int16

	// Current events
	currentEvents int16

	error error
}

// Returns the current error or nil
func (s *OnCallBackConfig) InError() error {
	return s.error
}

// Returns true if this this job is in a timeout.  If the job is in an error state error is not nil.
func (s *OnCallBackConfig) InTimeout() bool {
	return s.error == os.ErrDeadlineExceeded
}

// Returns true if the job is ready to read.
func (s *OnCallBackConfig) IsRead() bool {
	return s.error == nil && s.currentEvents&CAN_READ != 0
}

// Returns true if the job is ready to write.
func (s *OnCallBackConfig) IsWrite() bool {
	return s.error == nil && s.currentEvents&CAN_WRITE != 0
}

// Returns true if the job ready to read and write.
func (s *OnCallBackConfig) IsRW() bool {
	return s.error == nil && s.currentEvents&CAN_READ != 0 && s.events&CAN_WRITE != 0
}

// Sets the timeout for the listener in the current job.
func (s *OnCallBackConfig) SetTimeout(Timeout int64) {
	s.Timeout = Timeout
	if s.error == os.ErrDeadlineExceeded {
		s.error = nil
	}
}

// Tells the Current worker to stop polling this job and remove it from the thread pool.
func (s *OnCallBackConfig) Release() {
	s.events = CAN_END
	s.Timeout = 0
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

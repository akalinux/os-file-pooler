package osfp

import (
	"os"
	"time"
)

type CallbackEvent struct {
	nextTs int64

	// internal timeout config
	timeout int64
	// Update this value to change what events we poll
	events uint32

	// Current events
	currentEvents uint32

	error error

	now time.Time
}

// Returns the current error or nil
func (s *CallbackEvent) Error() error {
	return s.error
}

func (s *CallbackEvent) InError() bool {
	return s.error != nil
}

// Returns true if this this job is in a timeout.  If the job is in an error state error is not nil.
func (s *CallbackEvent) InTimeout() bool {
	return s.error == os.ErrDeadlineExceeded
}

// Returns true if the job is ready to read.
func (s *CallbackEvent) IsRead() bool {
	return s.error == nil && s.currentEvents&CAN_READ != 0
}

// Returns true if the job is ready to write.
func (s *CallbackEvent) IsWrite() bool {
	return s.error == nil && s.currentEvents&CAN_WRITE != 0
}

// Returns true if the job ready to read and write.
func (s *CallbackEvent) IsRW() bool {
	return s.error == nil && s.currentEvents&CAN_READ != 0 && s.events&CAN_WRITE != 0
}

// Sets the timeout for the listener in the current job.
func (s *CallbackEvent) SetTimeout(Timeout int64) {
	s.timeout = Timeout
	if s.error == os.ErrDeadlineExceeded {
		s.error = nil
	}
}

// Tells the Current worker to stop polling this job and remove it from the thread pool.
func (s *CallbackEvent) Release() {
	s.events = CAN_END
	s.timeout = 0
}

// Tells current worker to poll reads for this job.
func (s *CallbackEvent) PollRead() {
	s.events = CAN_READ
}

// Tells current worker poll writes for this job.
func (s *CallbackEvent) PollWrite() {
	s.events = CAN_WRITE
}

// Tells current worker poll both read and write for this job.
func (s *CallbackEvent) PollReadWrite() {
	s.events = CAN_RW
}

// Returns the curren time
func (s *CallbackEvent) GetNow() time.Time {
	return s.now
}

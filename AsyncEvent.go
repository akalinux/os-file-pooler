package osfp

import "time"

type AsyncEvent interface {
	// Returns the current error if any
	Error() error

	// Returns true if the current error is not nil
	InError() bool

	// Returns true if the curent error is a timeout
	InTimeout() bool

	// Returns true if this is a read event
	IsRead() bool

	// Returns true if this is a write event
	IsWrite() bool

	// Returns true if this is both a read and write event
	IsRW() bool

	// Updates the timeout, this value will be added to the current time in milliseconds.
	SetTimeout(int64)

	// Releases this Job on completion of the current event.
	Release()

	// Sets the job to read polling mode.
	PollRead()

	// Sets the job to write poll mode
	PollWrite()

	// Sets the job to read/write polling.
	PollReadWrite()

	// Gets the current timestamp used by the itnernals.
	GetNow() time.Time

	// Gets the current timeout that has been set.
	GetTimeout() int64

	// Overloads the current error value.
	SetError(error)
}

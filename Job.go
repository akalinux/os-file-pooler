package osfp

import (
	"sync/atomic"
)

// Internal job sequence id
var jobIdSeq *int64

func init() {
	var i int64 = -1
	jobIdSeq = &i
}

const JOB_ID_ADD_ONE int64 = 1

// Thread safe function used to generate new JobIds
func NextJobId() int64 {
	return atomic.AddInt64(jobIdSeq, JOB_ID_ADD_ONE)
}

// This is the core Job interface used by the Worker internals.
//
// # Events
//
// The "watchEvents" value represents the events we want to watch.
// The "currentEvents" value represents the events that were found.
//
// Supported Events:
//   - osfp.CAN_READ, denotes a read poll
//   - osfp.CAN_WRITE, denotes a write poll
//
// Both event types can be put togeather:
//
//	// Poll reads
//	watchEvents :=osfp.CAN_READ
//
//	// Poll write
//	watchEvents :=osfp.CAN_WRITE
//
//	// Poll both read and write
//	watchEvents :=osfp.CAN_WRITE|osfp.CAN_READ
//
// # Timeouts
//
// The "futureTimeOut" is expected to be unix timestamp in milliseconds that represents when the "watchEvents"
// polling window has expired. If "futureTimeOut" is less than or equal to 0, then the polling window will never expire.
//
// # Fd==-1
//
// Jobs that have an unix Fd of -1 do not have a file to poll and simply act as timers to be watched in the pool.
type Job interface {

	// Processes the last epoll events and returns the next flags to use.
	ProcessEvents(currentEvents uint32, now int64) (watchEevents uint32, futureTimeOut int64, EventError error)

	// Called to validate the "lastTimeout", should return a futureTimeOut or 0 if there is no timeout.
	// If the Job has timed out TimeOutError should be set to os.ErrDeadlineExceeded.
	CheckTimeOut(now, lastTimeout int64) (WatchEvents uint32, futureTimeOut int64, TimeOutError error)

	// Sets the current Worker. This method is called when a Job is added to a Worker in the pool.
	// A timeout only based job is a job where the fd returned is -1.
	SetPool(worker *Worker, now int64) (watchEevents uint32, futureTimeOut int64, fd int32)

	// This is called when Job is being removed from the pool.
	// Make sure to remove the refernce of the current worker when implementing this method.
	// The error value is nil if the "watchEvents" value is 0 and no errors were found.
	ClearPool(error)

	Release() (message error)

	// Returns the internal id for the job
	JobId() int64

	// Returns the unix fd
	Fd() int32
}

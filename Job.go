package osfp

type Job interface {
	// Expected to return the required Epoll flags.
	// When the return value is 0, the job will not be
	// added to the pool.
	GetFlags() int

	// Processes the last epoll flags and returns the next flags to use.
	// If the nextFlags value is 0, then the job will not be re-added.
	// The "futureTimeout" is expected unix stamp that represents when this job expires.
	// If "futureTimeout" is set to 0, then this job never expires.
	// If the timeout is greater than 0 but less than now.. then the object will be sutdown
	// with an os.ErrDeadlineExceeded error.
	ProcessFlags(flags int16, now int64) (nextFlags int16, nextFutureTimeOut int64)

	// notes that this job needs to shutdown.
	Shutdown(error)

	// Sets the current Worker. This method is called when a Job is added to a Worker in the pool.
	// The "futureTimeout" is expected unix stamp that represents when this job expires.
	// If "futureTimeout" is set to 0, then this job never expires.
	// If the timeout is greater than 0 but less than now.. then the object will be sutdown
	// with an os.ErrDeadlineExceeded error.
	// The "fd" value is expected to be the same as the value from os.File.Fd().  The Fd value
	// represents the unix file descriptor.
	SetPool(worker *Worker) (futureTimeout int64, fd int)

	// This is called when Job is being removed from the pool.
	// Make sure to remove the refernce of the current worker when implementing this method.
	ClearPool()
}

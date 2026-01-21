package osfp

// Common interface shared between both Worker and Pool
type PoolOrWorkerContainer interface {
	// Add a given job to either a thread pool or a worker.
	AddJob(job Job) (err error)

	// stops the given pool or worker, there was a problem if err is not nil
	Stop() (err error)
}

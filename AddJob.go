package osfp

// Common interface shared between both Worker and Pool
type PoolOrWorkerContainer interface {
	AddJob(job Job) (err error)
	Stop() (err error)
}

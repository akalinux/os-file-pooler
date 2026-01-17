package osfp

// Internal Container
type wjc struct {
	// last last unix timestamp in ms
	nextTs int64
	Job
	wanted uint32
}

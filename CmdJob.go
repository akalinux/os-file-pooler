package osfp

import "os"

type CmdJob struct {
	*WaitPidJob
	*os.Process
}

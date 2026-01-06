package osfp

import "os"

type SigWatchJob struct {
	*CallBackJob
	File *os.File
}

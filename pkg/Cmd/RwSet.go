package cmd

import "os"

type RwSet struct {
	Read  *os.File
	Write *os.File
}

package osfp

import (
	"os"
	"testing"
)

func Pipe() (r, w *os.File) {
	var e error
	r, w, e = os.Pipe()
	if e != nil {
		panic(e)
	}
	return
}
func TestNewWorker(t *testing.T) {

	w, e := NewStandAloneWorker()
	if e != nil {
		t.Errorf("Failed to start New local worker")
	}

	if w.JobCount() != 0 {
		t.Errorf("Expected 0 elements, got: %d", w.JobCount())
	}

	// should not throw errors
	w.Close()
	w.Close()
	w.Close()

}

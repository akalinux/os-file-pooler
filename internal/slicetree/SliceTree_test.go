package slicetree

import (
	"cmp"
	"testing"
)

func TestEmpty(t *testing.T) {
	t.Logf("starting")
	s := NewSliceTree(1, func(a, b int) int {
		return cmp.Compare(a, b)
	})

	if got := s.GetIndex(0); got != -2 {
		t.Fatalf("Index should be -2,got %d", got)
	}
	t.Logf("Cap is: %d", cap(s.Slice))
	s.Slice = append(s.Slice, 1)
	if got := s.GetIndex(0); got != -1 {
		t.Fatalf("Index should be -1,got %d", got)
	}

}

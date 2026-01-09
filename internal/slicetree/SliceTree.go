package slicetree

import (
	"fmt"
	"slices"
)

type SliceTree[T any] struct {

	// Internally managed slice
	Slice []T

	// Compare function.
	Cmp func(a, b T) int

	//  Tells the internals how far to go with indexing, the internals will scan any remaining chunk less than this value.
	// If set to any number less than 2, scanning is disabled and we will use btree style indexing for everything.
	// The recommended value is 3.
	ScanAt int

	// Required non 0 value, determins by what capacity we grow the internal
	// Slice.  Default is 1.
	Grow int

	// Required non nil value, called when ever a value is overwritten
	OnOverrite func(oldValue, newValue T)
}

func LookString(nextBegin, nextEnd, nextMid, offset int, resolved bool) string {
	return fmt.Sprintf("nextBegin: %d, nextEnd: %d, nextMid: %d, offset: %d, resolved: %v", nextBegin, nextEnd, nextMid, offset, resolved)
}

func StubOnOverwrite[T any](old, new T) {}
func NewSliceTree[T any](size int, cb func(a, b T) int) *SliceTree[T] {
	return &SliceTree[T]{
		Slice:      make([]T, 0, size),
		Cmp:        cb,
		ScanAt:     3,
		Grow:       1,
		OnOverrite: StubOnOverwrite[T],
	}
}

func (s *SliceTree[T]) GetMid(size int) int {
	return (size-2)/2 + size&1
}

func (s *SliceTree[T]) ClearBefore(v T) []T {
	return nil
}

func (s *SliceTree[T]) ClearAfter(v T) []T {
	return nil
}

func (s *SliceTree[T]) ClearTo(v T) []T {
	return nil
}
func (s *SliceTree[T]) ClearFrom(v T) []T {
	return nil
}

func (s *SliceTree[T]) Remove(v T) bool {
	return false
}

func (s *SliceTree[T]) Add(v T) (index int) {
	total := cap(s.Slice)

	if total == 0 {
		// 0 size.. just append
		s.Slice = append(s.Slice, v)
		return 0
	}
	idx, offset := s.GetIndex(v)
	return s.setIdx(idx, offset, v)
}

func (s *SliceTree[T]) setIdx(idx, offset int, v T) (index int) {
	size := len(s.Slice)
	if offset != 0 {
		ns := size + 1
		s.grow(ns)
		s.Slice = append(s.Slice, v)
		switch idx {
		case size:
			if offset == 1 {
				// add new value
				s.Slice = append(s.Slice, v)
				return ns
			} else {
				// move the old value
				s.Slice = append(s.Slice, s.Slice[idx])
				// set the new value
				s.Slice[idx] = v
				return idx
			}
		case 0:
			if offset == 1 {
				copy(s.Slice[2:], s.Slice[1:size])
				s.Slice[1] = v
				return 1
			} else {
				copy(s.Slice[1:], s.Slice[0:size])
				s.Slice[0] = v
			}
			return 0
		default:
			index = idx + offset
			copy(s.Slice[index+1:], s.Slice[index:size])
			if offset < 0 {
				s.Slice[idx] = v
				return idx
			} else {
				s.Slice[index] = v
				return index
			}

		}
	} else {
		s.OnOverrite(s.Slice[idx], v)
		s.Slice[idx] = v
		return idx
	}
}

func (s *SliceTree[T]) grow(size int) {
	if cap(s.Slice) < size {

		s.Slice = slices.Grow(s.Slice, s.Grow)
	}

}

func (s *SliceTree[T]) GetIndex(v T) (index, offset int) {

	size := len(s.Slice)
	if size == 0 {
		// set is empty!
		return 0, 1
	}
	nextMid := s.GetMid(size)
	if nextMid == -1 {
		// only one element
		return 0, s.Cmp(v, s.Slice[0])
	}

	// well if we get here.. we need to walk the tree
	nextBegin := 0
	nextEnd := len(s.Slice) - 1
	var resolved bool

	for {
		nextBegin, nextEnd, nextMid, offset, resolved = s.ResolveNext(nextBegin, nextEnd, nextMid, v)
		if resolved {
			index = offset + nextMid
			offset = s.Cmp(v, s.Slice[index])
			break
		}

	}

	return
}

func (s *SliceTree[T]) LookAhead(begin, end, mid int, v T) (nextBegin, nextEnd, nextMid, offset int, resolved bool) {
	nextBegin = mid + 1
	diff := end - nextBegin

	if diff <= 0 {
		resolved = true
		nextMid = mid
		offset = 1
		return
	} else if diff < s.ScanAt {
		resolved = true
		nextMid, offset = s.ScanSet(begin, nextEnd, v)
		return
	}
	nextMid = nextBegin + s.GetMid(diff+1)
	nextEnd = end
	offset = s.Cmp(s.Slice[nextMid], v)
	resolved = offset == 0
	return
}

func (s *SliceTree[T]) ScanSet(begin, end int, v T) (offset, index int) {
	for i := begin; i <= end; i++ {

		offset = s.Cmp(v, s.Slice[i])
		index = i

		if offset < 1 {
			// if we got here, then the value is either equal to or before this value
			return
		}
	}
	return
}

func (s *SliceTree[T]) LookBehind(begin, end, mid int, v T) (nextBegin, nextEnd, nextMid, offset int, resolved bool) {
	nextEnd = mid - 1
	diff := nextEnd - begin

	if diff <= 0 {
		resolved = true
		nextMid = mid
		offset = -1
		return
	} else if diff < s.ScanAt {
		resolved = true
		nextMid, offset = s.ScanSet(begin, nextEnd, v)
		return
	}
	nextMid = begin + s.GetMid(diff+1)
	nextBegin = begin
	offset = s.Cmp(s.Slice[nextMid], v)
	resolved = offset == 0
	return
}

func (s *SliceTree[T]) ResolveNext(begin, end, mid int, v T) (nextBegin, nextEnd, nextMid, offset int, resolved bool) {

	cmp := s.Cmp(v, s.Slice[mid])
	switch cmp {
	case 0:
		nextMid = mid
		resolved = true
		offset = cmp
		return
	case -1:
		nextBegin, nextEnd, nextMid, offset, resolved = s.LookBehind(begin, end, mid, v)
	case 1:
		nextBegin, nextEnd, nextMid, offset, resolved = s.LookAhead(begin, end, mid, v)
	}

	return
}

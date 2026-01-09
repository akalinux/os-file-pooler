package slicetree

import (
	"fmt"
	"slices"
)

type SliceTree[K any, V any] struct {

	// Internally managed keys slice
	Keys []K

	// Internally managed values slice
	Values []V

	// Compare function.
	Cmp func(a, b K) int

	//  Tells the internals how far to go with indexing, the internals will scan any remaining chunk less than this value.
	// If set to any number less than 2, scanning is disabled and we will use btree style indexing for everything.
	// The recommended value is 3.
	ScanAt int

	// Required non 0 value, determins by what capacity we grow the internal
	// Slice.  Default is 1.
	Grow int

	// Required non nil value, called when ever a value is overwritten
	OnOverrite func(key K, oldValue V, newValue V)
}

func LookString(nextBegin, nextEnd, nextMid, offset int, resolved bool) string {
	return fmt.Sprintf("nextBegin: %d, nextEnd: %d, nextMid: %d, offset: %d, resolved: %v", nextBegin, nextEnd, nextMid, offset, resolved)
}

func StubOnOverwrite[K any, V any](key K, oldValue, newValue V) {}
func NewSliceTree[K any, V any](size int, cb func(a, b K) int) *SliceTree[K, V] {
	return &SliceTree[K, V]{
		Keys:       make([]K, 0, size),
		Values:     make([]V, 0, size),
		Cmp:        cb,
		ScanAt:     3,
		Grow:       100,
		OnOverrite: StubOnOverwrite[K, V],
	}
}

func (s *SliceTree[K, V]) GetMid(size int) int {
	return (size-2)/2 + size&1
}

func (s *SliceTree[K, V]) Remove(k K) bool {
	return false
}

func (s *SliceTree[K, T]) Add(k K, v T) (index int) {
	total := cap(s.Keys)

	if total == 0 {
		// 0 size.. just append
		s.Keys = append(s.Keys, k)
		s.Values = append(s.Values, v)
		return 0
	}
	idx, offset := s.GetIndex(k)
	return s.setIdx(idx, offset, k, v)
}

func (s *SliceTree[K, V]) setIdx(idx, offset int, k K, v V) (index int) {
	size := len(s.Keys)
	if offset != 0 {
		ns := size + 1
		s.grow(ns)
		s.Keys = append(s.Keys, k)
		s.Values = append(s.Values, v)
		switch idx {
		case size:
			if offset == 1 {
				// add new value
				s.Keys = append(s.Keys, k)
				s.Values = append(s.Values, v)
				return ns
			} else {
				// move the old value
				s.Keys = append(s.Keys, s.Keys[idx])
				s.Values = append(s.Values, s.Values[idx])
				// set the new value
				s.Keys[idx] = k
				s.Values[idx] = v
				return idx
			}
		case 0:
			if offset == 1 {
				copy(s.Keys[2:], s.Keys[1:size])
				s.Keys[1] = k
				copy(s.Values[2:], s.Values[1:size])
				s.Values[1] = v
				return 1
			} else {
				copy(s.Keys[1:], s.Keys[0:size])
				s.Keys[0] = k
				copy(s.Values[1:], s.Values[0:size])
				s.Values[0] = v
			}
			return 0
		default:
			index = idx + offset
			copy(s.Keys[index+1:], s.Keys[index:size])
			copy(s.Values[index+1:], s.Values[index:size])
			if offset < 0 {
				s.Keys[idx] = k
				s.Values[idx] = v
				return idx
			} else {
				s.Keys[index] = k
				s.Values[index] = v
				return index
			}

		}
	} else {
		ns := idx + 1
		if size < ns {
			// empty slice!
			s.grow(ns)
			s.Keys = s.Keys[:ns]
			s.Values = s.Values[:ns]
		}
		s.OnOverrite(k, s.Values[idx], v)

		s.Keys[idx] = k
		s.Values[idx] = v
		return idx
	}
}

func (s *SliceTree[K, V]) grow(size int) {
	if cap(s.Keys) < size {

		s.Keys = slices.Grow(s.Keys, s.Grow)
		s.Values = slices.Grow(s.Values, s.Grow)
	}

}

func (s *SliceTree[K, V]) GetIndex(k K) (index, offset int) {

	size := len(s.Keys)
	if size == 0 {
		// set is empty!
		return 0, 1
	}
	nextMid := s.GetMid(size)
	if nextMid == -1 {
		// only one element
		return 0, s.Cmp(k, s.Keys[0])
	}

	// well if we get here.. we need to walk the tree
	nextBegin := 0
	nextEnd := len(s.Keys) - 1
	var resolved bool

	for {
		nextBegin, nextEnd, nextMid, offset, resolved = s.ResolveNext(nextBegin, nextEnd, nextMid, k)
		if resolved {
			index = offset + nextMid
			offset = s.Cmp(k, s.Keys[index])
			break
		}

	}

	return
}

func (s *SliceTree[K, V]) LookAhead(begin, end, mid int, k K) (nextBegin, nextEnd, nextMid, offset int, resolved bool) {
	nextBegin = mid + 1
	diff := end - nextBegin

	if diff <= 0 {
		resolved = true
		nextMid = mid
		offset = 1
		return
	} else if diff < s.ScanAt {
		resolved = true
		nextMid, offset = s.ScanSet(begin, nextEnd, k)
		return
	}
	nextMid = nextBegin + s.GetMid(diff+1)
	nextEnd = end
	offset = s.Cmp(s.Keys[nextMid], k)
	resolved = offset == 0
	return
}

func (s *SliceTree[K, V]) ScanSet(begin, end int, k K) (offset, index int) {
	for i := begin; i <= end; i++ {

		offset = s.Cmp(k, s.Keys[i])
		index = i

		if offset < 1 {
			// if we got here, then the value is either equal to or before this value
			return
		}
	}
	return
}

func (s *SliceTree[K, V]) LookBehind(begin, end, mid int, k K) (nextBegin, nextEnd, nextMid, offset int, resolved bool) {
	nextEnd = mid - 1
	diff := nextEnd - begin

	if diff <= 0 {
		resolved = true
		nextMid = mid
		offset = -1
		return
	} else if diff < s.ScanAt {
		resolved = true
		nextMid, offset = s.ScanSet(begin, nextEnd, k)
		return
	}
	nextMid = begin + s.GetMid(diff+1)
	nextBegin = begin
	offset = s.Cmp(s.Keys[nextMid], k)
	resolved = offset == 0
	return
}

func (s *SliceTree[K, V]) ResolveNext(begin, end, mid int, k K) (nextBegin, nextEnd, nextMid, offset int, resolved bool) {

	cmp := s.Cmp(k, s.Keys[mid])
	switch cmp {
	case 0:
		nextMid = mid
		resolved = true
		offset = cmp
		return
	case -1:
		nextBegin, nextEnd, nextMid, offset, resolved = s.LookBehind(begin, end, mid, k)
	case 1:
		nextBegin, nextEnd, nextMid, offset, resolved = s.LookAhead(begin, end, mid, k)
	}

	return
}

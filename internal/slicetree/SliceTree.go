package slicetree

import (
	"cmp"
	"iter"
	"slices"
)

type SliceTree[K any, V any] struct {

	// Internally managed keys slice
	Slices []*KvSet[K, V]

	// Compare function.
	Cmp func(a, b K) int

	// Required non 0 value, determins by what capacity we grow the internal
	// Slice.  Default is 1.
	Growth int

	// Required non nil value, called when ever a value is overwritten
	OnOverrite func(key K, oldValue V, newValue V) V
}

// Stub overwrite method, used by the constructor.  This is the default callback used when a value is overwritten.
func stubOnOverwrite[K any, V any](key K, oldValue, newValue V) V { return newValue }

// Creatss a new SliceTree with the internal Slice set to "size".
func NewSliceTree[K any, V any](size int, cb func(a, b K) int) *SliceTree[K, V] {
	return &SliceTree[K, V]{
		Slices:     make([]*KvSet[K, V], 0, size),
		Cmp:        cb,
		Growth:     100,
		OnOverrite: stubOnOverwrite[K, V],
	}
}

// Creates a new SliceTee with the default Slices size of 100.
func New[K any, V any](cb func(a, b K) int) *SliceTree[K, V] {
	return NewSliceTree[K, V](100, cb)
}

func (s *SliceTree[K, V]) getMid(size int) int {
	return (size-2)/2 + size&1
}

// Tries to remove the element of k, returns false if it fails.
// Complexity: o(log n)
func (s *SliceTree[K, V]) Remove(k K) bool {

	idx, offset := s.GetIndex(k)
	return s.clearIdx(idx, offset)
}

// Sets the key/vale pair and returns the index id.
// Comlexity: o(log n)
func (s *SliceTree[K, T]) Put(k K, v T) (index int) {
	total := len(s.Slices)

	if total == 0 {
		// 0 size.. just append
		s.Slices = append(s.Slices, &KvSet[K, T]{k, v})
		return 0
	}
	idx, offset := s.GetIndex(k)
	return s.setIdx(idx, offset, k, v)
}

// Tries to fetch value based on key of k, if k does not exist, found is false.
func (s *SliceTree[K, V]) Get(k K) (value V, found bool) {
	if len(s.Slices) == 0 {
		return
	}
	i, o := s.GetIndex(k)
	if o == 0 {
		return s.Slices[i].Value, true
	}
	return
}

// Returns true if the k exists in the slcie.
// Complexity: o(log n)
func (s *SliceTree[K, V]) Exists(k K) bool {
	size := len(s.Slices)
	switch size {
	case 0:
		return false
	case 1:
		return s.Cmp(s.Slices[0].Key, k) == 0
	}
	_, o := s.GetIndex(k)
	return o == 0
}

func (s *SliceTree[K, V]) clearIdx(idx, offset int) (result bool) {

	size := len(s.Slices)
	if offset != 0 || size == 0 || idx >= size || idx < 0 {
		result = false
	} else if size == 1 {
		// single element
		if idx == 0 {
			s.Slices = s.Slices[:0]
			return true
		}
	} else {
		s.Slices = slices.Delete(s.Slices, idx, idx+1)
		return true
	}

	return false
}

// Clears all elements in the slice, but keeps the memory allocated.
func (s *SliceTree[K, V]) ClearAll() int {
	t := len(s.Slices)
	s.Slices = s.Slices[:0]
	return t
}

// Returns the total number key/value pairs in the slice.
func (s *SliceTree[K, V]) Size() int {
	return len(s.Slices)
}

func (s *SliceTree[K, V]) setIdx(idx, offset int, k K, v V) (index int) {
	size := len(s.Slices)
	if offset != 0 {
		ns := size + 1
		s.grow(ns)
		s.Slices = append(s.Slices, nil)
		kv := &KvSet[K, V]{k, v}
		switch idx {
		case 0:
			if offset == 1 {
				copy(s.Slices[2:], s.Slices[1:size])
				s.Slices[1] = kv
				return 1
			} else {
				copy(s.Slices[1:], s.Slices[0:size])
				s.Slices[0] = kv
			}
			return 0
		default:
			index = idx + offset
			copy(s.Slices[index+1:], s.Slices[index:size])
			if offset < 0 {
				s.Slices[idx] = kv
				return idx
			} else {
				s.Slices[index] = kv
				return index
			}

		}
	} else {
		ns := idx + 1
		if size < ns {
			// empty slice!
			s.grow(ns)
			s.Slices = s.Slices[:ns]
			s.Slices[idx] = &KvSet[K, V]{k, v}
		} else {
			// overwrite
			s.Slices[idx].Value = s.OnOverrite(k, s.Slices[idx].Value, v)
		}

		return idx
	}
}

func (s *SliceTree[K, V]) grow(size int) {
	if cap(s.Slices) < size {
		s.Slices = slices.Grow(s.Slices, s.Growth)
	}
}

// Returns the index and offset.
//
// The index is the current postion in the slice.
//
// The offset represents where the item would be placed:
//   - offset of 0, at index value.
//   - offset of 1, expand the slice and put the value to the right of the index
//   - offset of -1, expand the slice and put the value to left of the current postion
//
// Complexity: o(log n)
func (s *SliceTree[K, V]) GetIndex(k K) (index, offset int) {

	size := len(s.Slices)
	switch size {
	case 0:
		return 0, 0
	case 1:
		return 0, s.Cmp(k, s.Slices[0].Key)
	}
	nextMid := s.getMid(size)
	// well if we get here.. we need to walk the tree
	nextBegin := 0
	nextEnd := len(s.Slices) - 1
	var resolved bool

	for {
		nextBegin, nextEnd, nextMid, offset, resolved = s.resolveNext(nextBegin, nextEnd, nextMid, k)
		if resolved {
			index = offset + nextMid
			if index < 0 {
				return nextMid, offset
			}
			offset = s.Cmp(k, s.Slices[index].Key)
			break
		}

	}

	return
}

func (s *SliceTree[K, V]) lookAhead(end, mid int, k K) (nextBegin, nextEnd, nextMid, offset int, resolved bool) {
	nextBegin = mid + 1
	diff := end - nextBegin

	if diff <= 0 {
		resolved = true
		nextMid = mid
		offset = 1
		return
	}
	nextMid = nextBegin + s.getMid(diff+1)
	nextEnd = end
	offset = s.Cmp(s.Slices[nextMid].Key, k)
	resolved = offset == 0
	return
}

func (s *SliceTree[K, V]) lookBehind(begin, mid int, k K) (nextBegin, nextEnd, nextMid, offset int, resolved bool) {
	nextEnd = mid - 1
	diff := nextEnd - begin

	if diff <= 0 {
		resolved = true
		nextMid = mid
		offset = -1
		return
	}
	nextMid = begin + s.getMid(diff+1)
	nextBegin = begin
	offset = s.Cmp(s.Slices[nextMid].Key, k)
	resolved = offset == 0
	return
}

func (s *SliceTree[K, V]) resolveNext(begin, end, mid int, k K) (nextBegin, nextEnd, nextMid, offset int, resolved bool) {

	cmp := s.Cmp(k, s.Slices[mid].Key)
	switch cmp {
	case 0:
		nextMid = mid
		resolved = true
		offset = cmp
		return
	case -1:
		nextBegin, nextEnd, nextMid, offset, resolved = s.lookBehind(begin, mid, k)
	case 1:
		nextBegin, nextEnd, nextMid, offset, resolved = s.lookAhead(end, mid, k)
	}

	return
}

// Returns an iterator for the current keys.
func (s *SliceTree[K, V]) Keys() iter.Seq2[int, K] {
	pos := 0
	return func(yield func(int, K) bool) {
		for pos < len(s.Slices) {
			if !yield(pos, s.Slices[pos].Key) {
				return
			}
			pos++
		}
	}
}

// Returns an iterator for the current values
func (s *SliceTree[K, V]) Values() iter.Seq2[int, V] {

	pos := 0
	return func(yield func(int, V) bool) {
		for pos < len(s.Slices) {
			if !yield(pos, s.Slices[pos].Value) {
				return
			}
			pos++
		}
	}
}

// Returns an iterator for key/value pars.
func (s *SliceTree[K, V]) All() iter.Seq2[K, V] {
	pos := 0
	return func(yield func(K, V) bool) {
		for pos < len(s.Slices) {
			kv := s.Slices[pos]
			if !yield(kv.Key, kv.Value) {
				return
			}
			pos++
		}
	}
}

// Attempts to remove the keys from the tree in bulk.  Returns the number of keys removed.
//
// Complexity: o(log n)*2 +1.
//
// The complexity is defined by the steps required:
//   - Index lookups: o(log n)
//   - child index is created to de-duplicate and order keys for deletion: o(log n)
//   - key deletion is done in contiguous blocks: +1
func (s *SliceTree[K, V]) MassRemove(args ...K) (total int) {
	if len(s.Slices) == 0 {
		return 0
	}
	f := New[int, any](reverse)
	for _, k := range args {
		i, o := s.GetIndex(k)
		if o != 0 {
			continue
		}
		f.Put(i, nil)
	}

	total = f.Size()
	s.contig(total, f.Keys(), s.rangedel)

	return
}

// This method is by defenition, unsafe!
//
// Only use if the keys being removed meet allo of the following requirements:
//   - no duplicate keys
//   - keys are in ascending ordered
//   - all keys currently exist
//
// Complexity o(log n)+1
func (s *SliceTree[K, V]) UnSafeMassRemove(keys ...K) {
	s.contig(len(keys), s.unsafeIter(keys), s.rangedel)
}

func (s *SliceTree[K, V]) unsafeIter(keys []K) iter.Seq2[int, int] {
	pos := len(keys) - 1
	id := 0
	return func(yield func(int, int) bool) {
		for pos > -1 {
			i, _ := s.GetIndex(keys[pos])
			if !yield(id, i) {
				return
			}
			id++
			pos--
		}
	}

}

func (s *SliceTree[K, V]) rangedel(a, b int) {
	s.Slices = slices.Delete(s.Slices, a, b+1)
}

func reverse(a, b int) int {
	return cmp.Compare(b, a)
}

func (s *SliceTree[K, V]) contig(totalKeys int, r iter.Seq2[int, int], cb func(a, b int)) {
	var end = -1
	var last = -1
	size := totalKeys - 1
	for i, idx := range r {
		p := idx + 1
		if last == p {
			if end == -1 {
				end = last
			}
			if i == size {
				cb(idx, end)
			}

		} else if last != -1 && end == -1 {

			cb(last, last)
			if i == size {
				cb(idx, idx)
			}
		} else {
			if end != -1 {
				cb(last, end)
			} else if i == size {
				cb(idx, idx)
			}
			end = -1
		}

		last = idx
	}
}

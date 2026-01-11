package slicetree

import (
	"iter"
	"sync"
)

type ThreadSafeOrderedMap[K any, V any] struct {
	// Instance to wrap for locking
	Tree *SliceTree[K, V]
	lock sync.RWMutex
}

// Creates a new thread safe OrderedMap.
func NewTs[K any, V any](Cmp func(a, b K) int) (Map OrderedMap[K, V]) {

	Map = &ThreadSafeOrderedMap[K, V]{
		Tree: New[K, V](Cmp),
	}
	return
}

// All implements [OrderedMap].
func (s *ThreadSafeOrderedMap[K, V]) All() iter.Seq2[K, V] {
	return func(yield func(K, V) bool) {
		pos := 0
		s.lock.RLock()
		defer s.lock.RUnlock()
		for {
			k, v, ok := s.nextKv(pos)
			if !ok {
				return
			}
			if !yield(k, v) {
				return
			}
			pos++
		}
	}
}

// ClearAfter implements [OrderedMap].
func (s *ThreadSafeOrderedMap[K, V]) ClearAfter(key K) (total int) {
	s.lock.Lock()
	defer s.lock.Unlock()
	return s.Tree.ClearAfter(key)
}

// ClearAfterS implements [OrderedMap].
func (s *ThreadSafeOrderedMap[K, V]) ClearAfterS(key K) (result []*KvSet[K, V]) {
	s.lock.Lock()
	defer s.lock.Unlock()
	return s.Tree.ClearAfterS(key)
}

// ClearAll implements [OrderedMap].
func (s *ThreadSafeOrderedMap[K, V]) ClearAll() int {
	s.lock.Lock()
	defer s.lock.Unlock()
	return s.Tree.ClearAll()
}

// ClearBefore implements [OrderedMap].
func (s *ThreadSafeOrderedMap[K, V]) ClearBefore(key K) (total int) {
	s.lock.Lock()
	defer s.lock.Unlock()
	return s.Tree.ClearBefore(key)
}

// ClearBeforeS implements [OrderedMap].
func (s *ThreadSafeOrderedMap[K, V]) ClearBeforeS(key K) (result []*KvSet[K, V]) {
	s.lock.Lock()
	defer s.lock.Unlock()
	return s.Tree.ClearBeforeS(key)
}

// Creates a thread safe iterator for a slice of *KvSet.
func TsKvIter[K any, V any](set []*KvSet[K, V]) iter.Seq2[K, V] {
	var lock sync.RWMutex
	return func(yield func(K, V) bool) {
		lock.RLock()
		defer lock.RUnlock()
		for _, row := range set {
			if !yield(row.Key, row.Value) {
				return
			}
		}
	}
}

// ClearBeforeI implements [OrderedMap].
// Returns a thread safe iterator for the deleted values.
func (s *ThreadSafeOrderedMap[K, V]) ClearBeforeI(key K) iter.Seq2[K, V] {
	return TsKvIter(s.ClearBeforeS(key))
}

// ClearFromI implements [OrderedMap].
// Returns a thread safe iterator for the deleted values.
func (s *ThreadSafeOrderedMap[K, V]) ClearFromI(key K) iter.Seq2[K, V] {
	return TsKvIter(s.ClearFromS(key))
}

// ClearAfterI implements [OrderedMap].
// Returns a thread safe iterator for the deleted values.
func (s *ThreadSafeOrderedMap[K, V]) ClearAfterI(key K) iter.Seq2[K, V] {
	return TsKvIter(s.ClearAfterS(key))
}

// ClearClearToI implements [OrderedMap].
// Returns a thread safe iterator for the deleted values.
func (s *ThreadSafeOrderedMap[K, V]) ClearToI(key K) iter.Seq2[K, V] {
	return TsKvIter(s.ClearToS(key))
}

// ClearFrom implements [OrderedMap].
func (s *ThreadSafeOrderedMap[K, V]) ClearFrom(key K) (total int) {
	s.lock.Lock()
	defer s.lock.Unlock()
	return s.Tree.ClearFrom(key)
}

// ClearFromS implements [OrderedMap].
func (s *ThreadSafeOrderedMap[K, V]) ClearFromS(key K) (result []*KvSet[K, V]) {
	s.lock.Lock()
	defer s.lock.Unlock()
	return s.Tree.ClearFromS(key)
}

// ClearTo implements [OrderedMap].
func (s *ThreadSafeOrderedMap[K, V]) ClearTo(key K) (total int) {
	s.lock.Lock()
	defer s.lock.Unlock()
	return s.Tree.ClearTo(key)
}

// ClearToS implements [OrderedMap].
func (s *ThreadSafeOrderedMap[K, V]) ClearToS(key K) (result []*KvSet[K, V]) {
	s.lock.Lock()
	defer s.lock.Unlock()
	return s.Tree.ClearToS(key)
}

// Exists implements [OrderedMap].
func (s *ThreadSafeOrderedMap[K, V]) Exists(key K) bool {
	s.lock.RLock()
	defer s.lock.RUnlock()
	return s.Tree.Exists(key)
}

// Get implements [OrderedMap].
func (s *ThreadSafeOrderedMap[K, V]) Get(key K) (value V, found bool) {
	s.lock.RLock()
	defer s.lock.RUnlock()
	return s.Tree.Get(key)
}

// Keys implements [OrderedMap].
func (s *ThreadSafeOrderedMap[K, V]) Keys() iter.Seq2[int, K] {
	return func(yield func(int, K) bool) {
		pos := 0
		s.lock.RLock()
		defer s.lock.RUnlock()

		for {
			k, _, ok := s.nextKv(pos)
			if !ok {
				return
			}
			if !yield(pos, k) {
				return
			}
			pos++
		}
	}
}

// MassRemove implements [OrderedMap].
func (s *ThreadSafeOrderedMap[K, V]) MassRemove(keys ...K) (total int) {
	s.lock.Lock()
	defer s.lock.Unlock()
	return s.Tree.MassRemove(keys...)
}

// Put implements [OrderedMap].
func (s *ThreadSafeOrderedMap[K, V]) Put(key K, value V) (index int) {
	s.lock.Lock()
	defer s.lock.Unlock()
	return s.Tree.Put(key, value)
}

// Remove implements [OrderedMap].
func (s *ThreadSafeOrderedMap[K, V]) Remove(key K) bool {
	s.lock.Lock()
	defer s.lock.Unlock()
	return s.Tree.Remove(key)
}

// Size implements [OrderedMap].
func (s *ThreadSafeOrderedMap[K, V]) Size() int {
	s.lock.RLock()
	defer s.lock.RUnlock()
	return s.Tree.Size()
}

// Values implements [OrderedMap].
func (s *ThreadSafeOrderedMap[K, V]) Values() iter.Seq2[int, V] {
	return func(yield func(int, V) bool) {
		pos := 0
		s.lock.RLock()
		defer s.lock.RUnlock()
		for {
			_, v, ok := s.nextKv(pos)
			if !ok {
				return
			}
			if !yield(pos, v) {
				return
			}
			pos++
		}
	}
}

// Always returns true.
func (s *ThreadSafeOrderedMap[K, V]) ThreadSafe() bool {
	return true
}

func (s *ThreadSafeOrderedMap[K, V]) nextKv(pos int) (key K, value V, ok bool) {

	if s.Tree.Size() > pos && pos > -1 {
		ok = true
		key = s.Tree.Slices[pos].Key
		value = s.Tree.Slices[pos].Value

	}
	return
}

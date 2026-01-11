package slicetree

import (
	"iter"
	"sync"
)

type ThreadSafeOrderedMap[K any, V any] struct {
	tree SliceTree[K, V]
	lock sync.RWMutex
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
	return s.tree.ClearAfter(key)
}

// ClearAfterS implements [OrderedMap].
func (s *ThreadSafeOrderedMap[K, V]) ClearAfterS(key K) (result []*KvSet[K, V]) {
	s.lock.Lock()
	defer s.lock.Unlock()
	return s.tree.ClearAfterS(key)
}

// ClearAll implements [OrderedMap].
func (s *ThreadSafeOrderedMap[K, V]) ClearAll() int {
	s.lock.Lock()
	defer s.lock.Unlock()
	return s.tree.ClearAll()
}

// ClearBefore implements [OrderedMap].
func (s *ThreadSafeOrderedMap[K, V]) ClearBefore(key K) (total int) {
	s.lock.Lock()
	defer s.lock.Unlock()
	return s.tree.ClearBefore(key)
}

// ClearBeforeS implements [OrderedMap].
func (s *ThreadSafeOrderedMap[K, V]) ClearBeforeS(key K) (result []*KvSet[K, V]) {
	s.lock.Lock()
	defer s.lock.Unlock()
	return s.tree.ClearBeforeS(key)
}

// ClearFrom implements [OrderedMap].
func (s *ThreadSafeOrderedMap[K, V]) ClearFrom(key K) (total int) {
	s.lock.Lock()
	defer s.lock.Unlock()
	return s.tree.ClearFrom(key)
}

// ClearFromS implements [OrderedMap].
func (s *ThreadSafeOrderedMap[K, V]) ClearFromS(key K) (result []*KvSet[K, V]) {
	s.lock.Lock()
	defer s.lock.Unlock()
	return s.tree.ClearFromS(key)
}

// ClearTo implements [OrderedMap].
func (s *ThreadSafeOrderedMap[K, V]) ClearTo(key K) (total int) {
	s.lock.Lock()
	defer s.lock.Unlock()
	return s.tree.ClearTo(key)
}

// ClearToS implements [OrderedMap].
func (s *ThreadSafeOrderedMap[K, V]) ClearToS(key K) (result []*KvSet[K, V]) {
	s.lock.Lock()
	defer s.lock.Unlock()
	return s.tree.ClearToS(key)
}

// Exists implements [OrderedMap].
func (s *ThreadSafeOrderedMap[K, V]) Exists(key K) bool {
	s.lock.RLock()
	defer s.lock.RUnlock()
	return s.tree.Exists(key)
}

// Get implements [OrderedMap].
func (s *ThreadSafeOrderedMap[K, V]) Get(key K) (value V, found bool) {
	s.lock.RLock()
	defer s.lock.RUnlock()
	return s.tree.Get(key)
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
	return s.tree.MassRemove(keys...)
}

// Put implements [OrderedMap].
func (s *ThreadSafeOrderedMap[K, V]) Put(key K, value V) (index int) {
	s.lock.Lock()
	defer s.lock.Unlock()
	return s.tree.Put(key, value)
}

// Remove implements [OrderedMap].
func (s *ThreadSafeOrderedMap[K, V]) Remove(key K) bool {
	s.lock.Lock()
	defer s.lock.Unlock()
	return s.tree.Remove(key)
}

// Size implements [OrderedMap].
func (s *ThreadSafeOrderedMap[K, V]) Size() int {
	s.lock.RLock()
	defer s.lock.RUnlock()
	return s.tree.Size()
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

func (s *ThreadSafeOrderedMap[K, V]) ThreadSafe() bool {
	return true
}

func (s *ThreadSafeOrderedMap[K, V]) nextKv(pos int) (key K, value V, ok bool) {

	if s.tree.Size() > pos && pos > -1 {
		ok = true
		key = s.tree.Slices[pos].Key
		value = s.tree.Slices[pos].Value

	}
	return
}

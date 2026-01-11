package slicetree

import (
	"cmp"
	"iter"
	"testing"
)

func TestLockIters(t *testing.T) {
	s := NewTs[int, int](cmp.Compare)
	for i := range 3 {
		s.Put(i, i)
	}

	for _, f := range []*KvSet[string, func() iter.Seq2[int, int]]{
		{"All", s.All},
		{"Keys", s.Keys},
		{"Values", s.Values},
	} {
		count := 0
		t.Logf("Testing loop iterator from: %s", f.Key)
		for range f.Value() {
			count++
		}
		if count != 3 {
			t.Fatalf("Failed to get 3 elements from %s, got: %d", f.Key, count)
		}
		count = 0
		for range f.Value() {
			count++
			break
		}
		if count != 1 {
			t.Fatalf("Failed to break at element: 0 elements from %s, got: %d", f.Key, count)
		}

	}

}
func TestClearIxxx(t *testing.T) {

	for i, b := range []func() OrderedMap[int, int]{
		func() OrderedMap[int, int] {
			return New[int, int](cmp.Compare)
		},
		func() OrderedMap[int, int] {
			return New[int, int](cmp.Compare).ToTs()
		},
	} {

		var name string
		switch i {
		case 0:
			name = ""
		case 1:
			name = "Ts-"
		}
		runIterCrc(t,
			&itercrc{
				name:    name + "ClearAfterI",
				size:    3,
				key:     1,
				removed: 1,
				cb: func(s OrderedMap[int, int], i int) iter.Seq2[int, int] {
					// Force this to be stested.
					// Should do nothing
					defer s.ClearAfter(i)
					return s.ClearAfterI(i)
				},
			},
			b,
		)
		runIterCrc(t,
			&itercrc{
				name:    name + "ClearBeforeI",
				size:    3,
				key:     1,
				removed: 1,
				cb: func(s OrderedMap[int, int], i int) iter.Seq2[int, int] {
					defer s.ClearBefore(i)
					return s.ClearBeforeI(i)
				},
			},
			b,
		)
		runIterCrc(t,
			&itercrc{
				name:    name + "ClearFromI",
				size:    3,
				key:     1,
				removed: 2,
				cb: func(s OrderedMap[int, int], i int) iter.Seq2[int, int] {
					defer s.ClearFrom(i)
					return s.ClearFromI(i)
				},
			},
			b,
		)
		runIterCrc(t,
			&itercrc{
				name:    name + "ClearToI",
				size:    3,
				key:     1,
				removed: 2,
				cb: func(s OrderedMap[int, int], i int) iter.Seq2[int, int] {
					defer s.ClearTo(i)
					return s.ClearToI(i)
				},
			},
			b,
		)
	}
}

func runIterCrc(t *testing.T, r *itercrc, f func() OrderedMap[int, int]) {
	s := f()
	t.Logf("Starting Test of [%s] is threadSafe: %v", r.name, s.ThreadSafe())
	for i := range r.size {
		s.Put(i, i+r.size)
		//t.Logf("  Key: %d value: %d", i, i+r.size)
	}
	t.Logf("  Size is: %d", s.Size())
	count := 0

	crc := r.size - r.removed
	t.Logf("  Key used: %d, Expected Remainig: %d, Expected Removed: %d", r.key, crc, r.removed)
	res := r.cb(s, r.key)
	for range res {
		count++
	}

	if count != r.removed {
		t.Fatalf("Expected Removed: %d, got: %d", r.removed, count)
	}
	// force break testing
	// but also pointing to the same key again should not modify our data!
	for range res {
		break
	}

	for _, i := range []KvSet[string, iter.Seq2[int, int]]{
		{"All", s.All()},
		{"Keys", s.Keys()},
		{"Values", s.Values()},
	} {

		count := 0
		for range i.Value {
			count++
		}
		if count != crc {
			t.Fatalf("Failed testing Method: [%s], expected: %d, got: %d", i.Key, crc, count)
		}
	}
	if diff := s.ClearAll(); diff != crc {
		t.Fatalf("ClearAll failed, expected: %d,got %d", crc, diff)
	}
	if s.MassRemove() != 0 {
		t.Fatalf("Testing MassRemove failed")
	}
	if s.Size() != 0 {
		t.Fatalf("Failed Size check")
	}
	if _, ok := s.Get(0); ok {
		t.Fatalf("Failed Get check, should have no elements")
	}
	if s.Remove(0) {
		t.Fatalf("Should fail to remove anything from an empty set!")
	}
	if s.Exists(0) {
		t.Fatalf("Should Never say something exists on an empty set!")
	}
}

type itercrc struct {
	name    string
	cb      func(OrderedMap[int, int], int) iter.Seq2[int, int]
	size    int
	removed int
	key     int
}

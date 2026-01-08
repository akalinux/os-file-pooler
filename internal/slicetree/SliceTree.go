package slicetree

type SliceTree[T any] struct {
	Slice []T
	Cmp   func(a, b T) int
}

func NewSliceTree[T any](size int, cb func(a, b T) int) *SliceTree[T] {
	return &SliceTree[T]{
		Slice: make([]T, 0, size),
		Cmp:   cb,
	}
}

func (s *SliceTree[T]) GetMid(size int) int {
	return size/2 + size&1
}

func (s *SliceTree[T]) ClearBefore(v T) {
}

func (s *SliceTree[T]) ClearAfter(v T) {
}

func (s *SliceTree[T]) ClearTo(v T) {
}

func (s *SliceTree[T]) GetIndex(v T) (idx int) {

	size := len(s.Slice)
	if size == 0 {
		// set is empty!
		return -2
	}
	mid := s.GetMid(size)
	if mid == size {
		return s.Cmp(v, s.Slice[0])
	}

	return
}

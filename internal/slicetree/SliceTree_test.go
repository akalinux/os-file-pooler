package slicetree

import (
	"cmp"
	"testing"
)

func TestMid(t *testing.T) {
	s := NewSliceTree(1, func(a, b int) int {
		return cmp.Compare(a, b)
	})
	if check := s.GetMid(3); check != 1 {
		t.Fatalf("Expected: 1, got : %d", check)
	}
	if check := s.GetMid(2); check != 0 {
		t.Fatalf("Expected: 0, got : %d", check)
	}
	if check := s.GetMid(1); check == -1 {
		t.Fatalf("Expected: -1, got : %d", check)
	}
	if check := s.GetMid(0); check == -2 {
		t.Fatalf("Expected: -2, got : %d", check)
	}
	if check := s.GetMid(10); check != 4 {
		t.Fatalf("Expected: 4, got : %d", check)
	}
	if check := s.GetMid(5); check != 2 {
		t.Fatalf("Expected: 2, got : %d", check)
	}
}

func TestScanAt(t *testing.T) {
	s := &SliceTree[int]{
		Cmp:    cmp.Compare[int],
		ScanAt: 1,
		Slice:  []int{2, 4, 6, 8, 10},
	}
	offset, index := s.ScanSet(0, 4, 5)
	if offset != -1 {
		t.Errorf("Expected offset of -1, got %d", offset)
	}
	if index != 2 {
		t.Errorf("Expected index of 2, got %d", index)
	}
	offset, index = s.ScanSet(2, 3, 6)
	if offset != 0 {
		t.Errorf("Expected offset of 0, got %d", offset)
	}
	if index != 2 {
		t.Errorf("Expected index of 2, got %d", index)
	}
}

func TestLookBehind(t *testing.T) {
	s := &SliceTree[int]{
		Cmp: cmp.Compare[int],
		// never Scan!
		ScanAt: -1,
		//           0  1  2  3  4   5   6
		Slice: []int{2, 4, 6, 8, 10, 12, 14},
	}
	t.Log("In block 1")
	nextBegin, nextEnd, nextMid, offset, resolved := s.LookBehind(0, 6, 3, 5)
	t.Log(LookString(nextBegin, nextEnd, nextMid, offset, resolved))

	if resolved {
		t.Error("should not be resolved")
	}
	if nextBegin != 0 {
		t.Errorf("Expected next begin: 0, got: %d", nextBegin)
	}
	if nextEnd != 2 {
		t.Errorf("Expected next end: 1, got: %d", nextEnd)
	}
	if offset != -1 {
		t.Errorf("Expected offset: -1, got: %d", offset)
	}
	if nextMid != 1 {
		t.Errorf("Expected mid: 1, got: %d", nextMid)
	}
	t.Log("In block 2")
	nextBegin, nextEnd, nextMid, offset, resolved = s.LookBehind(2, 2, 2, 5)
	t.Log(LookString(nextBegin, nextEnd, nextMid, offset, resolved))
	if !resolved {
		t.Fatalf("should be resolved")
	}
	if nextMid != 2 {
		t.Errorf("Expected next mid: 2, got: %d", nextMid)
	}

}

func TestLookAhead(t *testing.T) {
	s := &SliceTree[int]{
		Cmp: cmp.Compare[int],
		// never Scan!
		ScanAt: -1,
		//           0  1  2  3  4   5   6
		Slice: []int{2, 4, 6, 8, 10, 12, 14},
	}
	t.Log("In block 1")
	nextBegin, nextEnd, nextMid, offset, resolved := s.LookAhead(0, 6, 3, 9)
	t.Log(LookString(nextBegin, nextEnd, nextMid, offset, resolved))

	if resolved {
		t.Error("should not be resolved")
	}
	if nextBegin != 4 {
		t.Errorf("Expected next begin: 4, got: %d", nextBegin)
	}
	if nextEnd != 6 {
		t.Errorf("Expected next end: 6, got: %d", nextEnd)
	}
	if offset != 1 {
		t.Errorf("Expected offset: 1, got: %d", offset)
	}
	if nextMid != 5 {
		t.Errorf("Expected mid: 5, got: %d", nextMid)
	}
	t.Log("In block 2")
	nextBegin, nextEnd, nextMid, offset, resolved = s.LookAhead(4, 6, 5, 9)
	t.Log(LookString(nextBegin, nextEnd, nextMid, offset, resolved))
	if !resolved {
		t.Fatalf("should be resolved")
	}
	if nextMid != 5 {
		t.Errorf("Expected next mid: 5, got: %d", nextMid)
	}

}

func TestGetIn(t *testing.T) {

	s := &SliceTree[int]{
		Cmp: cmp.Compare[int],
		// never Scan!
		ScanAt: -1,
		//           0  1  2  3  4   5   6
		Slice: []int{2, 4, 6, 8, 10, 12, 14},
	}
	index, offset := s.GetIndex(12)
	t.Logf("Got index: %d, offset: %d", index, offset)
	if s.Slice[index+offset] != 12 {
		t.Fatalf("Failed to fetchg our indexed value")
	}
	index, offset = s.GetIndex(2)
	t.Logf("Got index: %d, offset: %d", index, offset)
	if s.Slice[index+offset] != 2 {
		t.Fatalf("Failed to fetchg our indexed value")
	}

	index, offset = s.GetIndex(14)
	t.Logf("Got index: %d, offset: %d", index, offset)
	if s.Slice[index+offset] != 14 {
		t.Fatalf("Failed to fetchg our indexed value")
	}

	index, offset = s.GetIndex(15)
	t.Logf("Got index: %d, offset: %d", index, offset)
	index, offset = s.GetIndex(1)
	t.Logf("Got index: %d, offset: %d", index, offset)
}

func TestIdxSet(t *testing.T) {
	s := NewSliceTree[int](0, cmp.Compare)

	s.Slice = append(s.Slice, 1)

	expected := []int{1, 2}
	s.setIdx(0, 1, 2)
	checkExpected(t, "Set 1", expected, s.Slice)

	expected = []int{0, 1, 2}
	s.setIdx(0, -1, 0)
	checkExpected(t, "Set 2", expected, s.Slice)

	expected = []int{0, 1, 2, 4}
	s.setIdx(2, 1, 4)
	checkExpected(t, "Set 3", expected, s.Slice)

	expected = []int{0, 1, 2, 3, 4}
	s.setIdx(2, 1, 3)
	checkExpected(t, "Set 4", expected, s.Slice)
	//                       3
	s.Slice = []int{0, 1, 2, 4}
	s.setIdx(3, -1, 3)
	checkExpected(t, "Set 5", expected, s.Slice)

}

func checkExpected(t *testing.T, set string, exp, got []int) {
	t.Logf("** Starting set: %s", set)
	if len(got) != len(exp) {
		t.Fatalf("Expected: length of: %d, got length of %d", len(exp), len(got))
	}
	for i, check := range exp {
		t.Logf("id: %d, expected: %d, got %d", i, check, got[i])
		if got[i] != check {
			t.Errorf("Missmatch id: %d, expected: %d, got %d", i, check, got[i])
		}
	}
}

package slicetree

import (
	"cmp"
	"fmt"
	"testing"
)

func LookString(nextBegin, nextEnd, nextMid, offset int, resolved bool) string {
	return fmt.Sprintf("nextBegin: %d, nextEnd: %d, nextMid: %d, offset: %d, resolved: %v", nextBegin, nextEnd, nextMid, offset, resolved)
}
func TestMid(t *testing.T) {
	s := NewSliceTree[int, int](1, func(a, b int) int {
		return cmp.Compare(a, b)
	})
	if check := s.getMid(3); check != 1 {
		t.Fatalf("Expected: 1, got : %d", check)
	}
	if check := s.getMid(2); check != 0 {
		t.Fatalf("Expected: 0, got : %d", check)
	}
	if check := s.getMid(1); check == -1 {
		t.Fatalf("Expected: -1, got : %d", check)
	}
	if check := s.getMid(0); check == -2 {
		t.Fatalf("Expected: -2, got : %d", check)
	}
	if check := s.getMid(10); check != 4 {
		t.Fatalf("Expected: 4, got : %d", check)
	}
	if check := s.getMid(5); check != 2 {
		t.Fatalf("Expected: 2, got : %d", check)
	}
}

func TestGetIndex(t *testing.T) {

	s := &SliceTree[int, int]{
		Cmp: cmp.Compare[int],
		//           0  1  2  3  4   5   6
		Slices: []*KvSet[int, int]{
			{2, 0},
			{4, 0},
			{6, 0},
			{8, 0},
			{10, 0},
			{12, 0},
			{14, 0},
		},
	}
	index, offset := s.GetIndex(12)
	t.Logf("Got index: %d, offset: %d", index, offset)
	if s.Slices[index+offset].Key != 12 {
		t.Fatalf("Failed to fetchg our indexed value, expected: 12, got %d", s.Slices[index+offset].Key)
	}
	index, offset = s.GetIndex(2)
	t.Logf("Got index: %d, offset: %d", index, offset)
	if s.Slices[index+offset].Key != 2 {
		t.Fatalf("Failed to fetchg our indexed value")
	}

	index, offset = s.GetIndex(14)
	t.Logf("Got index: %d, offset: %d", index, offset)
	if s.Slices[index+offset].Key != 14 {
		t.Fatalf("Failed to fetchg our indexed value")
	}

	index, offset = s.GetIndex(15)
	t.Logf("Got index: %d, offset: %d", index, offset)
	index, offset = s.GetIndex(1)
	t.Logf("Got index: %d, offset: %d", index, offset)

	s = &SliceTree[int, int]{
		Cmp: cmp.Compare[int],
		//           0  1  2  3  4   5   6
		Slices: []*KvSet[int, int]{
			{0, 0},
			{1, 0},
		},
	}
	index, offset = s.GetIndex(2)
	t.Logf("Got index: %d, offset: %d", index, offset)

	s.Slices = []*KvSet[int, int]{{0, 0}}
	t.Logf("Root: %v, size: %d", s.Slices[0], len(s.Slices))
	// note this was fatal in one variation of the code
	index, offset = s.GetIndex(1)

	s.Slices = []*KvSet[int, int]{
		{0, 0},
		{1, 0},
		{2, 0},
		{3, 0},
	}
	index, offset = s.GetIndex(4)
	t.Logf("Index: %d, Offset: %d", index, offset)
	if index+offset != 4 {
		t.Fatalf("Expected: 4, got: %d", index+offset)
	}

}

func TestIdxSet(t *testing.T) {
	s := NewSliceTree[int, int](0, cmp.Compare)

	t.Log("Setting inital set of 0,0,1,0")
	expected := []int{1}
	s.SetIndex(0, 0, 1, 0)
	checkExpected(t, "Set 0", expected, s.Slices)

	expected = []int{1, 2}
	s.SetIndex(0, 1, 2, 0)
	checkExpected(t, "Set 1", expected, s.Slices)

	expected = []int{0, 1, 2}
	s.SetIndex(0, -1, 0, 0)
	checkExpected(t, "Set 2", expected, s.Slices)

	expected = []int{0, 1, 2, 4}
	s.SetIndex(2, 1, 4, 0)
	checkExpected(t, "Set 3", expected, s.Slices)

	expected = []int{0, 1, 2, 3, 4}
	s.SetIndex(2, 1, 3, 0)
	checkExpected(t, "Set 4", expected, s.Slices)
	// found a bug freom this test set
	s.Slices = []*KvSet[int, int]{
		{0, 0},
		{1, 0},
		{2, 0},
		{4, 0},
	}
	s.SetIndex(3, -1, 3, 0)
	checkExpected(t, "Set 5", expected, s.Slices)

}

func checkExpected(t *testing.T, set string, exp []int, got []*KvSet[int, int]) {
	t.Logf("** Starting set: %s", set)
	if len(got) != len(exp) {
		t.Fatalf("Expected: length of: %d, got length of %d", len(exp), len(got))
	}
	for i, check := range exp {
		t.Logf("id: %d, expected: %d, got %d", i, check, got[i])
		if got[i].Key != check {
			t.Fatalf("Missmatch id: %d, expected: %d, got %d", i, check, got[i])
		}
	}
}

func TestClearIndex(t *testing.T) {
	s := &SliceTree[int, int]{
		Cmp: cmp.Compare[int],
		Slices: []*KvSet[int, int]{
			{-1, 0},
			{0, 0},
			{1, 0},
			{2, 0},
			{3, 0},
			{4, 0},
		},
	}
	if s.clearIdx(-1, 0) {
		t.Fatalf("Should not be able to clear a negative index")
	}
	if s.clearIdx(len(s.Slices), 0) {
		t.Fatalf("Should not be able to clear an index value beyond our bounds")
	}
	if s.clearIdx(0, 1) {
		t.Fatalf("Should not be able to clear an offset position")
	}
	if s.clearIdx(0, -1) {
		t.Fatalf("Should not be able to clear an offset position")
	}

	fullCheck(t, "Delete first element", []*KvSet[int, int]{
		{0, 0},
		{1, 0},
		{2, 0},
		{3, 0},
		{4, 0},
	}, s.Slices, true, s.clearIdx(0, 0))
	fullCheck(t, "Delete last element", []*KvSet[int, int]{
		{0, 0},
		{1, 0},
		{2, 0},
		{3, 0},
	}, s.Slices, true, s.clearIdx(4, 0))

	fullCheck(t, "Delete 2nd element", []*KvSet[int, int]{
		{0, 0},
		{2, 0},
		{3, 0},
	}, s.Slices, true, s.clearIdx(1, 0))

	fullCheck(t, "Clear all", []*KvSet[int, int]{}, s.Slices, true, 3 == s.ClearAll())

	if s.Size() != 0 {
		t.Fatalf("Failed to actually clear our set!")
	}
	s.Slices = []*KvSet[int, int]{{0, 0}}
	s.clearIdx(0, 0)
	if len(s.Slices) != 0 {
		t.Fatalf("Slice should now be empty")
	}

}

func fullCheck(t *testing.T, test string, got, exp []*KvSet[int, int], state, res bool) {

	t.Logf("** Testing Set: [%s]", test)
	if state != res {
		t.Fatalf("Unexpected outcome")
	}
	for idx, check := range exp {
		if got[idx].Key != check.Key {
			t.Fatalf("Failed at clearing index on element: %d expected %v, got %v", idx, check, got[idx])
		}
	}
}

func TestPut(t *testing.T) {
	s := New[int, int](cmp.Compare)

	expected := []*KvSet[int, int]{}
	for i := range 10 {
		t.Logf("*** Setting idx: %d, key: %d", i, i)
		showAll(t, s.Slices)
		if len(s.Slices) != i {
			t.Fatalf("Expected len: %d, got: %d", i, len(s.Slices))
		}
		index, offset := s.GetIndex(i)
		t.Logf("Will add: %d at: %d, offset %d", i, index, offset)
		idx := s.Put(i, 0)
		if idx != i {
			showAll(t, s.Slices)
			t.Fatalf("Expected index: %d, got: %d", i, idx)
		}
		expected = append(expected, &KvSet[int, int]{i, 0})
	}
	fullCheck(t, "Set 0-9 in sequence", expected, s.Slices, true, true)

	s = New[int, int](cmp.Compare)
	for _, key := range []int{9, 8, 7, 6, 5, 4, 3, 2, 1, 0} {
		t.Logf("*** Setting idx: key: %d", key)
		showAll(t, s.Slices)
		t.Logf("Trying to index: %d", key)
		index, offset := s.GetIndex(key)
		t.Logf("Will add: %d at: %d, offset %d", key, index, offset)
		s.Put(key, 0)
	}
	fullCheck(t, "Set 9-0 in sequence", expected, s.Slices, true, true)

	s = New[int, int](cmp.Compare)
	for _, key := range []int{8, 9, 7, 3, 5, 4, 6, 0, 1, 2} {
		t.Logf("*** Setting idx: key: %d", key)
		showAll(t, s.Slices)
		t.Logf("Trying to index: %d", key)
		index, offset := s.GetIndex(key)
		t.Logf("Will add: %d at: %d, offset %d", key, index, offset)
		s.Put(key, 0)
	}
	fullCheck(t, "Set 0-9 in semi random order", expected, s.Slices, true, true)

	s = New[int, int](cmp.Compare)
	for _, key := range []int{67, 84, 54, 66, 187, 11, 0, 1, 2, 3, 11, 245} {
		t.Logf("*** Setting idx: key: %d", key)
		showAll(t, s.Slices)
		t.Logf("Trying to index: %d", key)
		index, offset := s.GetIndex(key)
		t.Logf("Will add: %d at: %d, offset %d", key, index, offset)
		s.Put(key, 0)
	}
	expected = []*KvSet[int, int]{
		{0, 0},
		{1, 0},
		{2, 0},
		{3, 0},
		{11, 0},
		{54, 0},
		{66, 0},
		{67, 0},
		{84, 0},
		{187, 0},
		{245, 0},
	}
	fullCheck(t, "Set 0-245 in semi random order, with gaps", expected, s.Slices, true, true)

}

func showAll(t *testing.T, list []*KvSet[int, int]) {
	for id, v := range list {
		t.Logf("   Idx: %d, Value: %v", id, v)
	}
}

func TestRemove(t *testing.T) {

	s := New[int, int](cmp.Compare)
	if s.Remove(0) {
		t.Fatalf("Should not remove anything")
	}
	s.Put(0, 0)
	if !s.Remove(0) {
		t.Fatalf("Should have removed our only element!")
	}
}

func TestGet(t *testing.T) {

	s := New[int, int](cmp.Compare)
	_, ok := s.Get(1)
	if ok {
		t.Fatalf("Invalid value")
	}
	v := s.Put(1, 2)
	v, ok = s.Get(11)
	if ok || v != 0 {
		t.Fatalf("Invalid value")
	}
	v, ok = s.Get(1)
	if !ok {
		t.Fatalf("Should have value key 1")
	}
	if v != 2 {
		t.Fatalf("Expected: 2, Got: %d", v)
	}

}

func TestIters(t *testing.T) {

	s := New[int, int](cmp.Compare)
	for i := range 3 {
		s.Put(i, i+3)
	}
	for i, k := range s.Keys() {
		if k != i {
			t.Fatalf("Failed Keys Test on Element: %d, expected: %d, got %d", i, i, k)
		}
		v, ok := s.Get(i)
		if !ok {
			t.Fatalf("Failed fetching value on on Element: %d", i)
		}
		if v != i+3 {
			t.Fatalf("Failed fetching value from key on Element: %d, expected: %d, got: %d", i, i+3, v)
		}
	}
	for i, v := range s.Values() {
		if i+3 != v {
			t.Fatalf("Bad Value id: %d, expected: %d, got: %d", i, i+3, v)
		}
	}

	id := 0
	for k, v := range s.All() {
		if id != k {
			t.Fatalf("Failed key test on index: %d, expected %d, got %d", id, id, k)
		}
		if v != id+3 {
			t.Fatalf("Failed value test on index: %d, expected %d, got %d", id, id+3, v)
		}
		id++
	}
	// yeild callack tests

	for range s.Keys() {
		break
	}
	for range s.Values() {
		break
	}
	for range s.All() {
		break
	}
}

func TestExists(t *testing.T) {
	s := New[int, int](cmp.Compare)
	if s.Exists(0) {
		t.Fatalf("No elements should exist!")
	}
	s.Put(11, 12)
	if s.Exists(0) {
		t.Fatalf("No Should not exist!")
	}
	if !s.Exists(11) {
		t.Fatalf("Should exist!")
	}
	// more than 1 element causes us to check the internal index
	s.Put(1, 2)
	if !s.Exists(11) {
		t.Fatalf("Should exist!")
	}
}

func TestContig(t *testing.T) {
	s := New[int, int](cmp.Compare)
	for i := range 20 {
		s.Put(i, 0)
	}

	f := New[int, any](reverse)
	//                      1  2  3  4  5   6   7
	for _, k := range []int{0, 3, 7, 8, 9, 10, 13} {
		i, o := s.GetIndex(k)
		if o != 0 {
			continue
		}
		f.Put(i, nil)
	}

	expected := [][]int{
		{13, 13},
		{7, 10},
		{3, 3},
		{0, 0},
	}

	contigCheck(t, "Set 1", expected, f)
	f.Slices = []*KvSet[int, any]{{0, nil}}
	contigCheck(t, "Set 2", [][]int{{0, 0}}, f)
	f.Slices = []*KvSet[int, any]{{1, nil}, {0, nil}}
	contigCheck(t, "Set 3", [][]int{{0, 1}}, f)
	f.Slices = []*KvSet[int, any]{
		{13, 0}, {12, 0},
		{1, nil}, {0, nil},
	}
	contigCheck(t, "Set 4", [][]int{{12, 13}, {0, 1}}, f)

}

func TestMassRemove(t *testing.T) {
	s := New[int, int](cmp.Compare)
	for i := range 15 {
		s.Put(i, 0)
	}
	if c := s.MassRemove(0, 3, 7, 8, 9, 10, 13); c != 7 {
		t.Fatalf("Expected a total of: 6 to be removed, got: %d", c)
	}

	for i, v := range []int{1, 2, 4, 5, 6, 11, 12, 14} {
		t.Logf("Trying record: %d, value %d", i, v)
		if s.Slices[i].Key != v {
			t.Fatalf("Failed to delete record: %d, expected key: %d, got: %d", i, v, s.Slices[i].Key)
		}
	}
	s = New[int, int](cmp.Compare)
	// empty and no args should do nothing!
	if 0 != s.MassRemove() {
		t.Fatalf("Emty remove should do nothing")
	}
	s.Put(1, 1)
	if 0 != s.MassRemove(0) {
		t.Fatalf("Remving a element that does not exist should do nothing")
	}
	s.MassRemove(1)
	if len(s.Slices) != 0 {
		t.Fatalf("Should be empty")
	}
	s = New[int, int](cmp.Compare)
	for i := range 15 {
		s.Put(i, 0)
	}
}

func TestUnsafeRemove(t *testing.T) {
	s := New[int, int](cmp.Compare)
	for i := range 15 {
		s.Put(i, 0)
	}

	s.UnSafeMassRemove(0, 3, 7, 8, 9, 10, 13)

	for i, v := range []int{1, 2, 4, 5, 6, 11, 12, 14} {
		t.Logf("Trying record: %d, value %d", i, v)
		if s.Slices[i].Key != v {
			t.Fatalf("Failed to delete record: %d, expected key: %d, got: %d", i, v, s.Slices[i].Key)
		}
	}
	// validate the inernals work as expected
	for range s.unsafeIter([]int{0}) {
		break
	}
}

func contigCheck(t *testing.T, name string, exp [][]int, f *SliceTree[int, any]) {
	t.Logf("Validating test: [%s]", name)
	got := make([][]int, len(exp))
	i := 0
	f.contig(f.Size(), f.Keys(), func(a, b int) {
		t.Logf("a: %d, b, %d", a, b)
		got[i] = append(got[i], a, b)
		i++
	})
	if len(exp) != len(got) {
		t.Fatalf("Wrong result size, expected: %d, got %d", len(exp), len(got))
	}
	for i, r := range exp {
		if len(got[i]) != 2 {
			t.Fatalf("Never got a record for id: %d", i)
		}
		if r[0] != got[i][0] {
			t.Fatalf("Bad Start, on set: %d, expected: %d, got %d", i, r[0], got[i][0])
		}
		if r[1] != got[i][1] {
			t.Fatalf("Bad End, on set: %d, expected: %d, got %d", i, r[1], got[i][1])
		}
	}
}

func TestClearTo(t *testing.T) {
	clearTesting(t, "Delete from mid",
		3, // total keys, 0,1,2
		1, // delete to
		2, // total removed
		[]int{2},
	)
	clearTesting(t, "Delete to end",
		3, // total keys, 0,1,2
		2, // delete to
		3, // total removed
		[]int{},
	)

	clearTesting(t, "Delete before the start",
		3,  // total keys, 0,1,2
		-1, // delete to
		0,  // total removed
		[]int{0, 1, 2},
	)
	clearTesting(t, "Delete after the end",
		3, // total keys, 0,1,2
		3, // delete to
		3, // total removed
		[]int{},
	)
}

func TestClearToS(t *testing.T) {
	clearTestingS(t, "Delete from mid",
		3,           // total keys, 0,1,2
		1,           // delete to
		[]int{0, 1}, // total removed
		[]int{2},
	)
	clearTestingS(t, "Delete to end",
		3,              // total keys, 0,1,2
		2,              // delete to
		[]int{0, 1, 2}, // total removed
		[]int{},
	)

	clearTestingS(t, "Delete before the start",
		3,  // total keys, 0,1,2
		-1, // delete to
		[]int{},
		[]int{0, 1, 2},
	)
	clearTestingS(t, "Delete after the end",
		3, // total keys, 0,1,2
		3, // delete to
		[]int{0, 1, 2},
		[]int{},
	)
}

func clearTestingS(t *testing.T, test string, keys, to int, removed []int, res []int, opt ...int) {
	t.Logf("Running test set: [%s]", test)
	s := New[int, int](cmp.Compare)
	for i := range keys {
		t.Logf("  Adding key: %d", i)
		s.Put(i, 0)
	}
	for i, kv := range s.Slices {
		t.Logf("  Inital Row: %d, value: %d", i, kv.Key)
	}
	var check []*KvSet[int, int]
	if len(opt) == 0 {
		t.Logf("Using ClearToS")
		check = s.ClearToS(to)
		// calling again should do nothing
		s.ClearToS(to)
	} else if opt[0] == 1 {
		t.Logf("Using ClearFromS")
		check = s.ClearFromS(to)
		s.ClearFromS(to)
	} else if opt[0] == 2 {
		t.Logf("Using ClearBeforeS")
		check = s.ClearBeforeS(to)
		s.ClearBeforeS(to)
	} else if opt[0] == 3 {
		t.Logf("Using ClearAfterS")
		check = s.ClearAfterS(to)
		s.ClearAfterS(to)
	}

	for i, kv := range s.Slices {
		t.Logf("  Final Row: %d, value: %d", i, kv.Key)
	}
	if len(check) != len(removed) {
		t.Fatalf("Expected delete size of: %d, got: %d", len(removed), len(check))
	}
	for i, k := range removed {
		if check[i].Key != k {
			t.Fatalf("Bad delete record, expected: %d, got %d for row: %d", k, check[i].Key, i)
		}
	}
	if len(s.Slices) != len(res) {
		t.Fatalf("Expected s.Slices len: %d, got %d", len(res), len(s.Slices))
	}
	for i, k := range res {
		if s.Slices[i].Key != k {
			t.Fatalf("Bad internal state, for record: %d, expected: %d, got: %d", i, k, s.Slices[i].Key)
		}
	}

}

func clearTesting(t *testing.T, test string, keys, to, removed int, res []int, opt ...int) {
	t.Logf("Running test set: [%s]", test)
	s := New[int, int](cmp.Compare)
	for i := range keys {
		t.Logf("  Adding key: %d", i)
		s.Put(i, 0)
	}
	for i, kv := range s.Slices {
		t.Logf("  Inital Row: %d, value: %d", i, kv.Key)
	}
	var check int
	if len(opt) == 0 {
		t.Logf("Calling ClearTo")
		check = s.ClearTo(to)
	} else if opt[0] == 1 {
		t.Logf("Calling ClearBefore")
		check = s.ClearBefore(to)
	} else if opt[0] == 2 {
		t.Logf("Calling ClearFrom")
		check = s.ClearFrom(to)
	} else if opt[0] == 3 {
		t.Logf("Calling ClearAfter")
		check = s.ClearAfter(to)
	}

	for i, kv := range s.Slices {
		t.Logf("  Final Row: %d, value: %d", i, kv.Key)
	}
	if check != removed {
		t.Fatalf("Expected delete size of: %d, got: %d", removed, check)
	}
	if len(s.Slices) != len(res) {
		t.Fatalf("Expected s.Slices len: %d, got %d", len(res), len(s.Slices))
	}
	for i, k := range res {
		if s.Slices[i].Key != k {
			t.Fatalf("Bad internal state, for record: %d, expected: %d, got: %d", i, k, s.Slices[i].Key)
		}
	}

}

func TestClearBefore(t *testing.T) {
	clearTesting(t, "Delete from mid",
		3, // total keys, 0,1,2
		1, // delete to
		1, // total removed
		[]int{1, 2},
		1,
	)
	clearTesting(t, "Delete before the start",
		3,  // total keys, 0,1,2
		-1, // delete to
		0,  // total removed
		[]int{0, 1, 2},
		1,
	)
	clearTesting(t, "Delete after the end",
		3, // total keys, 0,1,2
		4, // delete to
		3, // total removed
		[]int{},
		1,
	)

}

func TestClearFrom(t *testing.T) {
	clearTesting(t, "Delete from mid",
		3, // total keys, 0,1,2
		1, // delete from
		2, // total removed
		[]int{0},
		2,
	)
	clearTesting(t, "Delete before the start",
		3,  // total keys, 0,1,2
		-1, // delete from
		3,  // total removed
		[]int{},
		2,
	)
	clearTesting(t, "Delete after the end",
		3, // total keys, 0,1,2
		4, // delete from
		0, // total removed
		[]int{0, 1, 2},
		2,
	)
}

func TestClearAfter(t *testing.T) {
	clearTesting(t, "Delete from mid",
		3, // total keys, 0,1,2
		1, // delete from
		1, // total removed
		[]int{0, 1},
		3,
	)
	clearTesting(t, "Delete before the start",
		3,  // total keys, 0,1,2
		-1, // delete from
		3,  // total removed
		[]int{},
		3,
	)
	clearTesting(t, "Delete after the end",
		3, // total keys, 0,1,2
		4, // delete from
		0, // total removed
		[]int{0, 1, 2},
		3,
	)
}
func TestClearFromS(t *testing.T) {
	clearTestingS(t, "Delete from mid",
		3,           // total keys, 0,1,2
		1,           // delete from
		[]int{1, 2}, // total removed
		[]int{0},
		1,
	)
	clearTestingS(t, "Delete to end",
		3,        // total keys, 0,1,2
		2,        // delete from
		[]int{2}, // total removed
		[]int{0, 1},
		1,
	)

	clearTestingS(t, "Delete before the start",
		3,              // total keys, 0,1,2
		-1,             // delete to
		[]int{0, 1, 2}, // removed
		[]int{},        // remaining
		1,
	)
	clearTestingS(t, "Delete after the end",
		3, // total keys, 0,1,2
		3, // delete to
		[]int{},
		[]int{0, 1, 2},
		1,
	)
}

func TestClearBeforeS(t *testing.T) {
	clearTestingS(t, "Delete from mid",
		3,        // total keys, 0,1,2
		1,        // delete before
		[]int{0}, // total removed
		[]int{1, 2},
		2,
	)
	clearTestingS(t, "Delete to end",
		3,           // total keys, 0,1,2
		2,           // delete before
		[]int{0, 1}, // keys removed
		[]int{2},    // keys remaining
		2,
	)

	clearTestingS(t, "Delete before the start",
		3,              // total keys, 0,1,2
		-1,             // delete before
		[]int{},        // removed
		[]int{0, 1, 2}, // remaining
		2,
	)
	clearTestingS(t, "Delete after the end",
		3,              // total keys, 0,1,2
		3,              // delete before
		[]int{0, 1, 2}, // removed
		[]int{},
		2,
	)
}

func TestClearAfterS(t *testing.T) {
	clearTestingS(t, "Delete from mid",
		3,        // total keys, 0,1,2
		1,        // delete after
		[]int{2}, // total removed
		[]int{0, 1},
		3,
	)
	clearTestingS(t, "Delete to end",
		3,       // total keys, 0,1,2
		2,       // delete after
		[]int{}, // total removed
		[]int{0, 1, 2},
		3,
	)

	clearTestingS(t, "Delete before the start",
		3,              // total keys, 0,1,2
		-1,             // delete after
		[]int{0, 1, 2}, // removed
		[]int{},        // remaining
		3,
	)
	clearTestingS(t, "Delete after the end",
		3, // total keys, 0,1,2
		3, // delete to
		[]int{},
		[]int{0, 1, 2},
		3,
	)
}

func TestSet(t *testing.T) {
	s := New[int, int](cmp.Compare)
	s.Put(0, 0)
	if !s.Set(0, 2) {
		t.Fatalf("should have set index 0 to 2")
	}
	v, ok := s.Get(0)
	if !ok {
		t.Fatalf("Should have been able to fetch index of 0")
	}
	if v != 2 {
		t.Fatalf("Should have been able to fetch index of 0")
	}
	if s.Set(1, 2) {
		t.Fatalf("should fail to set 1 to 2")
	}
	if s.Set(-1, 2) {
		t.Fatalf("should fail to set -1 to 2")
	}
}

func TestCast(t *testing.T) {
	s := New[int, int](cmp.Compare)

	func(m OrderedMap[int, int]) {
		t.Log("If we get here, then the object conforms to the interface")
		if m.ThreadSafe() {
			t.Fatalf("The object should say it is thread safe!")
		}
	}(s)
}

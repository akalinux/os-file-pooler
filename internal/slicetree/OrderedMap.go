package slicetree

type OrderedMap[K any, V any] interface {
	Map[K, V]

	// Clears all elements less than or equal to the key.
	// Returns the number of elements removed.
	// The key is not required to exist.
	ClearTo(key K) (total int)

	// Clears all elements less than or equal to the key.
	// Returns a slice that contains the removed KvSet elements.
	// The key is not required to exist.
	ClearToS(key K) (result []*KvSet[K, V])

	// Clears all elements less than the key.
	// Returns the number of elements removed.
	// The key is not required to exist.
	ClearBefore(key K) (total int)

	// Clears all elements less than the key.
	// Returns a slice that contains the removed KvSet elements.
	// The key is not required to exist.
	ClearBeforeS(key K) (result []*KvSet[K, V])

	// Clears all elements greater than or equeal to the key.
	// Returns the number of elements removed.
	// The key is not required to exist.
	ClearFrom(key K) (total int)

	// Clears all elements greater than or queal to the key.
	// Returns a slice that contains the removed KvSet elements.
	// The key is not required to exist.
	ClearFromS(key K) (result []*KvSet[K, V])

	// Clears all elements less than the key.
	// Returns the number of elements removed.
	// The key does not need to exist
	ClearAfter(key K) (total int)

	// Clears all elements less than the key.
	// Returns a slice that contains the removed KvSet elements.
	// The key is not required to exist.
	ClearAfterS(key K) (result []*KvSet[K, V])
}

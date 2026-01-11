package slicetree

import "iter"

type Map[K any, V any] interface {
	// Should return an iterator for all key/value pairs
	All() iter.Seq2[K, V]

	// Returns all keys, the int just expected to be a sequential number
	Keys() iter.Seq2[int, K]

	// Returns all the values, the int is expected to be a sequential number
	Values() iter.Seq2[int, V]

	// Clears all elements.
	// Returns how many element were removed.
	ClearAll() int

	// Returns true if the key exists, false if it does not.
	Exists(key K) bool

	// Tries to get the value using the given key.
	// If the key exists, found is set to true, if the key does not exists then found is set to false.
	Get(key K) (value V, found bool)

	// Attempts to remove all keys, returns the number of keys removed.
	MassRemove(keys ...K) (total int)

	// Sets the key to the value, returns the index id.
	Put(key K, vvalue V) (index int)

	// Tries to remove the given key, returns false if the key does not exist.
	Remove(key K) bool

	// Sets the value at a given index point.
	Size() int

	// Should return true of this instance is thread safe, fakse if not.
	ThreadSafe() bool
}

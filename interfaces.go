package lru

// Cache defines the features of a cache
type Cache interface {
	// Close empties the cache, releases all resources
	Close()
	// Get retrieves the value at the specified key
	Get(key Key) (v any, ok bool, err error)
	// Len returns the current usage of the cache
	Len() (l int, err error)
	// Put inserts the value at the specified key, replacing any prior content
	Put(key Key, val any) (err error)
	// Remove evicts the key and its associated value
	Remove(key Key) (err error)
}

package lru

import "context"

// KeyVal associates a Key to a Value
type KeyVal struct {
	// Key requested to be retrieved
	Key Key
	// Value retrieved for the key, if found
	Value any
}

// CacheResult describes the outcome of attempting to retrieve the value at the key
type CacheResult struct {
	KeyVal
	// OK set to true indicates successful retrieval for the key
	OK bool
	// Err holds any errors encountered during retrieval of this key
	Err error
}

// Cache defines the features of a cache
type Cache interface {
	// Close empties the cache, releases all resources
	Close()
	// Get retrieves the value at the specified key
	Get(ctx context.Context, key Key) (v any, ok bool, err error)
	// GetBatch retrieves multiple keys at once
	GetBatch(ctx context.Context, keys []Key) ([]*CacheResult, error)
	// Len returns the current usage of the cache
	Len() (l int, err error)
	// Put inserts the value at the specified key, replacing any prior content
	Put(ctx context.Context, key Key, val any) (err error)
	// PutBatch inserts multiple key/values at once
	PutBatch(ctx context.Context, vals []KeyVal) (err error)
	// Remove evicts the key and its associated value
	Remove(key Key) (err error)

	// Added to prevent implementations outside this package, minimising impact of change
	private()
}

package lru

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// Loader is a func that returns the value for the key, or returns an error
type Loader func(key Key) (any, error)

// LoadingCache is an implementation of Cache that will attempt to populate
// itself for a missing Key, using a specified Loader function
type LoadingCache struct {
	cache  *BasicCache
	loader Loader
}

// Close empties the cache, releases all resources
func (l *LoadingCache) Close() {
	l.cache.Close()
}

// Get retrieves the value at the specified key
func (l *LoadingCache) Get(key Key) (any, bool, error) {

	v, ok, err := l.cache.Get(key)
	if err != nil {
		return nil, false, err
	}
	if ok {
		return v, ok, err
	}

	v, err = l.loader(key)
	if err != nil {
		return nil, false, err
	}

	err = l.Put(key, v)
	if err != nil {
		return nil, false, err
	}

	return v, true, nil
}

// Len returns the current usage of the cache
func (l *LoadingCache) Len() (int, error) {
	return l.cache.Len()
}

// Put inserts the value at the specified key, replacing any prior content
func (l *LoadingCache) Put(key Key, val any) (err error) {
	return l.cache.Put(key, val)
}

// Remove evicts the key and its associated value
func (l *LoadingCache) Remove(key Key) (err error) {
	return l.cache.Remove(key)
}

var ErrInvalidLoader = errors.New("loader must not be nil")

// NewLoadingCache creates a new LRU cache instance with the specified capacity
// and timeout for request processing, plus it will invoke the specified Loader function
// to populate the cache, if the requested key is not already in the cache.
// If capacity > 0 then a new addition will trigger eviction of the
// least recently used item.  If capacity = 0 then cache will grow
// indefinitely.
// If timeout <= 0 then an infinite timeout is used (not recommended)
// Close() should be called when the cache is no longer needed, to release resources
func NewLoadingCache(ctx context.Context, loader Loader, maxEntries int, timeout time.Duration) (*LoadingCache, error) {

	if loader == nil {
		return nil, ErrInvalidLoader
	}

	// Ensures recovery from panic, converted to error
	wrapped := func(key Key) (v any, err error) {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("unexpected error: %v", r)
			}
		}()

		return loader(key)
	}

	c, err := NewBasicCache(ctx, maxEntries, timeout)
	if err != nil {
		return nil, err
	}

	return &LoadingCache{
		cache:  c,
		loader: wrapped,
	}, nil
}

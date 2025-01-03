package lru

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// LoaderResult provides the outcome of an attempt to load the specified key
type LoaderResult struct {
	Key   Key
	Value any
	Err   error
}

// Loader is a func that returns the value for the specified keys
type Loader func(ctx context.Context, key []Key) ([]LoaderResult, error)

// LoadingCache is an implementation of Cache that will attempt to populate
// itself for a missing Key, using a specified Loader function
type LoadingCache struct {
	privateImp
	cache  *BasicCache
	loader Loader
}

// Close empties the cache, releases all resources
func (l *LoadingCache) Close() {
	l.cache.Close()
}

// Get retrieves the value at the specified key
func (l *LoadingCache) Get(ctx context.Context, key Key) (any, bool, error) {
	res, err := l.GetBatch(ctx, []Key{key})
	if err != nil {
		return nil, false, err
	}
	if len(res) == 0 {
		return nil, false, ErrUnknown
	}
	return res[0].Value, res[0].OK, res[0].Err
}

const (
	oTELLoadingCacheGetBatchStarted = "LoadingCache.GetBatch started"
	oTELLoadingCacheGetBatchEnded   = "LoadingCache.GetBatch ended"
	oTELLoadingCacheGetBatchError   = "LoadingCache.GetBatch Retrieval Error"
)

// GetBatch retrieves the values at the specified keys
func (l *LoadingCache) GetBatch(ctx context.Context, keys []Key) (res []*CacheResult, err error) {

	select {
	case <-ctx.Done():
		return nil, ErrInvalidContext
	default:
	}

	curSpan := trace.SpanFromContext(ctx)
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("unexpected error: %v", r)
			curSpan.AddEvent(oTELLoadingCacheGetBatchError, trace.WithTimestamp(time.Now().UTC()))
			curSpan.SetStatus(codes.Error, err.Error())
		} else {
			curSpan.AddEvent(oTELLoadingCacheGetBatchEnded, trace.WithAttributes(attribute.Int("Retrieved", len(res))), trace.WithTimestamp(time.Now().UTC()))
		}
	}()

	curSpan.AddEvent(oTELLoadingCacheGetBatchStarted, trace.WithAttributes(attribute.Int("Requested", len(keys))), trace.WithTimestamp(time.Now().UTC()))

	res, err = l.cache.GetBatch(ctx, keys)

	if err != nil {
		return []*CacheResult{}, err
	}
	if len(res) != len(keys) {
		return []*CacheResult{}, ErrUnknown
	}

	loaderKeys := []Key{}
	for _, r := range res {
		if r.Err != nil || !r.OK {
			loaderKeys = append(loaderKeys, r.Key)
		}
	}

	if len(loaderKeys) > 0 {

		loadResp, err := l.loader(ctx, loaderKeys)
		if err != nil {
			return []*CacheResult{}, err
		}
		if len(loadResp) != len(loaderKeys) {
			return []*CacheResult{}, ErrUnknown
		}

		toCache := []LoaderResult{}
		for _, lr := range loadResp {
			for _, cr := range res {
				if lr.Key == cr.Key {
					if lr.Err != nil {
						cr.Err = lr.Err
						cr.OK = false
					} else {
						cr.Value = lr.Value
						if cr.Value != nil {
							cr.OK = true
							toCache = append(toCache, lr)
						}
					}
					break
				}
			}
		}

		// No need to wait for cache to be updated
		go func() {
			defer recover() // No panics allowed

			for _, o := range toCache {
				l.Put(o.Key, o.Value)
			}
		}()

	}

	return res, nil
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

	select {
	case <-ctx.Done():
		return nil, ErrInvalidContext
	default:
	}

	if loader == nil {
		return nil, ErrInvalidLoader
	}

	// Ensures recovery from panic, converted to error
	wrapped := func(ctx context.Context, keys []Key) (cr []LoaderResult, err error) {

		curSpan := trace.SpanFromContext(ctx)
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("unexpected error: %v", r)
				curSpan.AddEvent(oTELLoaderError, trace.WithTimestamp(time.Now().UTC()))
				curSpan.SetStatus(codes.Error, err.Error())
			} else {
				curSpan.AddEvent(oTELLoaderEnded, trace.WithAttributes(attribute.Int("Loaded", len(cr))), trace.WithTimestamp(time.Now().UTC()))
			}
		}()

		curSpan.AddEvent(oTELLoaderStarted, trace.WithAttributes(attribute.Int("Requested", len(keys))), trace.WithTimestamp(time.Now().UTC()))

		cr, err = loader(ctx, keys)

		return
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

const (
	oTELLoaderStarted = "Loader started"
	oTELLoaderEnded   = "Loader ended"
	oTELLoaderError   = "Loader Error"
)

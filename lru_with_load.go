package lru

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.opentelemetry.io/otel/attribute"
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

// GetBatch retrieves the values at the specified keys
func (l *LoadingCache) GetBatch(ctx context.Context, keys []Key) ([]*CacheResult, error) {

	select {
	case <-ctx.Done():
		return nil, ErrInvalidContext
	default:
	}

	resp, err := l.cache.GetBatch(ctx, keys)

	if err != nil {
		return []*CacheResult{}, err
	}
	if len(resp) != len(keys) {
		return []*CacheResult{}, ErrUnknown
	}

	loaderKeys := []Key{}
	for _, r := range resp {
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
			for _, cr := range resp {
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

	return resp, nil
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
				curSpan.AddEvent(OTELSpanLoadErrorEvent, trace.WithAttributes(attribute.String("Error", err.Error())), trace.WithTimestamp(time.Now().UTC()))
			}
		}()

		curSpan.AddEvent(OTELSpanStartLoadEvent, trace.WithAttributes(attribute.Int("Requested", len(keys))), trace.WithTimestamp(time.Now().UTC()))

		cr, err = loader(ctx, keys)

		curSpan.AddEvent(OTELSpanEndLoadEvent, trace.WithAttributes(attribute.Int("Retrieved", len(cr))), trace.WithTimestamp(time.Now().UTC()))

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
	// OTELSpanStartLoadEvent is the name of the event created when data retrieval is requested from the loader, recording the number of keys
	OTELSpanStartLoadEvent = "Loading Data into Cache"
	// OTELSpanEndLoadEvent is the name of the event created when the loader completes, recording the number of retrievals
	OTELSpanEndLoadEvent = "Loaded Data into Cache"
	// OTELSpanLoadErrorEvent is the name of the event created if a panic occurs during loading
	OTELSpanLoadErrorEvent = "Cache Loading Error"
)

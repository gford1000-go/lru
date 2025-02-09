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

type removeRequest struct {
	k Key
	c chan struct{}
}

type putRequest struct {
	k Key
	v any
	c chan struct{}
}

type getRequest struct {
	keys []Key
	c    chan []*CacheResult
}

type getLenResponse struct {
	len int
}

type getLenRequest struct {
	c chan *getLenResponse
}

// BasicCache provides a concurrency-safe implementation
// of a bounded least-recently-used cache
type BasicCache struct {
	privateImp
	d   time.Duration
	put chan *putRequest
	get chan *getRequest
	rm  chan *removeRequest
	len chan *getLenRequest
}

// Close releases all resources associated with the cache
func (c *BasicCache) Close() {
	defer func() {
		recover()
	}()
	close(c.put)
	close(c.get)
	close(c.rm)
	close(c.len)
}

var ErrTimeout = errors.New("timeout exceeded")
var ErrUnknown = errors.New("unknown error")
var ErrAttemptToUseInvalidCache = errors.New("cache has been Closed() and is unusable")
var sendToClosedChanPanicMsg = "send on closed channel"

// Get will retrieve the item with the specified key
// into the cache, updating its lru status.
// An error is raised if the Close() has been called, or
// the timeoout for the operation is exceeded.
func (c *BasicCache) Get(ctx context.Context, key Key) (v any, ok bool, err error) {
	res, err := c.GetBatch(ctx, []Key{key})
	if err != nil {
		return nil, false, err
	}
	if len(res) == 0 {
		return nil, false, ErrUnknown
	}
	return res[0].Value, res[0].OK, res[0].Err
}

const (
	oTELBasicCacheGetBatchStarted = "BasicCache.GetBatch started"
	oTELBasicCacheGetBatchEnded   = "BasicCache.GetBatch ended"
	oTELBasicCacheGetBatchError   = "BasicCache.GetBatch Retrieval Error"
)

// GetBatch retrieves all the provided keys, returning a CacheResult for each
// one, which provides the details of the retrieval of the key
func (c *BasicCache) GetBatch(ctx context.Context, keys []Key) (cr []*CacheResult, err error) {

	select {
	case <-ctx.Done():
		return nil, ErrInvalidContext
	default:
	}

	curSpan := trace.SpanFromContext(ctx)
	defer func() {
		if r := recover(); r != nil {
			if fmt.Sprintf("%v", r) == sendToClosedChanPanicMsg {
				err = ErrAttemptToUseInvalidCache
			} else {
				err = fmt.Errorf("unexpected error: %v", r)
			}
			curSpan.AddEvent(oTELBasicCacheGetBatchError, trace.WithTimestamp(time.Now().UTC()))
			curSpan.SetStatus(codes.Error, err.Error())
		} else {
			curSpan.AddEvent(oTELBasicCacheGetBatchEnded, trace.WithAttributes(attribute.Int("Retrieved", len(cr))), trace.WithTimestamp(time.Now().UTC()))
		}
	}()

	curSpan.AddEvent(oTELBasicCacheGetBatchStarted, trace.WithAttributes(attribute.Int("Requested", len(keys))), trace.WithTimestamp(time.Now().UTC()))

	ch := make(chan []*CacheResult)
	defer close(ch)

	c.get <- &getRequest{
		keys: keys,
		c:    ch,
	}

	select {
	case <-ctx.Done():
		return nil, ErrInvalidContext
	case <-time.After(c.d):
		return nil, ErrTimeout
	case cr, ok := <-ch:
		if !ok {
			return nil, ErrUnknown
		}
		return cr, nil
	}
}

// Len returns the number of items in the cache
// An error is raised if the Close() has been called, or
// the timeoout for the operation is exceeded.
func (c *BasicCache) Len() (l int, err error) {
	defer func() {
		if r := recover(); r != nil {
			if fmt.Sprintf("%v", r) == sendToClosedChanPanicMsg {
				err = ErrAttemptToUseInvalidCache
			} else {
				// Something unexpected - report this
				err = fmt.Errorf("%v", r)
			}
		}
	}()

	ch := make(chan *getLenResponse)
	defer close(ch)

	c.len <- &getLenRequest{
		c: ch,
	}

	select {
	case <-time.After(c.d):
		return 0, ErrTimeout
	case r, ok := <-ch:
		if !ok {
			return 0, ErrUnknown
		}
		return r.len, nil
	}
}

// Put will insert the item with the specified key
// into the cache, replacing what was previously there (if anything).
// An error is raised if the Close() has been called, or
// the timeout for the operation is exceeded.
func (c *BasicCache) Put(ctx context.Context, key Key, val any) (err error) {
	return c.PutBatch(ctx, []KeyVal{{Key: key, Value: val}})
}

const (
	oTELBasicCachePutBatchStarted = "BasicCache.PutBatch started"
	oTELBasicCachePutBatchEnded   = "BasicCache.PutBatch ended"
	oTELBasicCachePutBatchError   = "BasicCache.PutBatch error"
)

var ErrInvalidValueToAddToCache = errors.New("value associated to a key cannot be nil")

// PutBatch will insert the items into the cache, replacing what was previously there (if anything).
// An error is raised if the Close() has been called, or the timeoout for the operation is exceeded.
func (c *BasicCache) PutBatch(ctx context.Context, vals []KeyVal) (err error) {

	select {
	case <-ctx.Done():
		return ErrInvalidContext
	default:
	}

	if len(vals) == 0 {
		return nil
	}

	var added = 0

	curSpan := trace.SpanFromContext(ctx)
	defer func() {
		if r := recover(); r != nil {
			if fmt.Sprintf("%v", r) == sendToClosedChanPanicMsg {
				err = ErrAttemptToUseInvalidCache
			} else {
				err = fmt.Errorf("unexpected error: %v", r)
			}
			curSpan.AddEvent(oTELBasicCachePutBatchError, trace.WithTimestamp(time.Now().UTC()))
			curSpan.SetStatus(codes.Error, err.Error())
		} else {
			curSpan.AddEvent(oTELBasicCachePutBatchEnded, trace.WithAttributes(attribute.Int("Added", added)), trace.WithTimestamp(time.Now().UTC()))
		}
	}()

	curSpan.AddEvent(oTELBasicCachePutBatchStarted, trace.WithAttributes(attribute.Int("Requested", len(vals))), trace.WithTimestamp(time.Now().UTC()))

	ch := make(chan struct{})
	defer close(ch)

	for _, v := range vals {

		if v.Value == nil {
			return ErrInvalidValueToAddToCache
		}

		c.put <- &putRequest{
			k: v.Key,
			v: v.Value,
			c: ch,
		}

		select {
		case <-ctx.Done():
			return ErrInvalidContext
		case <-time.After(c.d):
			return ErrTimeout
		case _, ok := <-ch:
			if !ok {
				return ErrUnknown
			}
			added++
		}
	}

	return nil
}

// Remove will remove the item with the specified key
// from the cache, ignoring if it does not exist.
// An error is raised if the Close() has been called, or
// the timeoout for the operation is exceeded.
func (c *BasicCache) Remove(key Key) (err error) {
	defer func() {
		if r := recover(); r != nil {
			if fmt.Sprintf("%v", r) == sendToClosedChanPanicMsg {
				err = ErrAttemptToUseInvalidCache
			} else {
				// Something unexpected - report this
				err = fmt.Errorf("%v", r)
			}
		}
	}()

	ch := make(chan struct{})
	defer close(ch)

	c.rm <- &removeRequest{
		k: key,
		c: ch,
	}

	select {
	case <-time.After(c.d):
		return ErrTimeout
	case _, ok := <-ch:
		if !ok {
			return ErrUnknown
		}
		return nil
	}
}

var ErrInvalidMaxEntries = errors.New("maxEntries must be zero or positive integer")

var ErrInvalidContext = errors.New("context has already ended")

// NewBasicCache creates a new LRU cache instance with the specified capacity
// and timeout for request processing.
// If capacity > 0 then a new addition will trigger eviction of the
// least recently used item.  If capacity = 0 then cache will grow
// indefinitely.
// If timeout <= 0 then an infinite timeout is used (not recommended)
// Close() should be called when the cache is no longer needed, to release resources
func NewBasicCache(ctx context.Context, maxEntries int, timeout time.Duration) (*BasicCache, error) {

	select {
	case <-ctx.Done():
		return nil, ErrInvalidContext
	default:
	}

	if maxEntries < 0 {
		return nil, ErrInvalidMaxEntries
	}

	if timeout <= 0 {
		timeout = time.Duration(24 * time.Hour) // Effectively infinite
	}

	c := &BasicCache{
		d:   timeout,
		get: make(chan *getRequest, 100),
		put: make(chan *putRequest, 100),
		rm:  make(chan *removeRequest, 100),
		len: make(chan *getLenRequest, 100),
	}

	go func() {
		cache := newCache(maxEntries)

		// Tidy up could take some time, so do this last
		defer cache.clear()
		// If exiting the routine, need to stop further requests
		// so call Close as this writes to the chans
		defer c.Close()

		for {
			select {
			case <-ctx.Done():
				return
			case r, ok := <-c.get:
				if !ok {
					return
				}
				resp := []*CacheResult{}
				for _, k := range r.keys {
					v, ok := cache.get(k)
					resp = append(resp, &CacheResult{
						KeyVal: KeyVal{
							Key:   k,
							Value: v,
						},
						OK: ok,
					})
				}
				r.c <- resp
			case r, ok := <-c.len:
				if !ok {
					return
				}
				v := cache.len()
				r.c <- &getLenResponse{
					len: v,
				}
			case r, ok := <-c.put:
				if !ok {
					return
				}
				cache.put(r.k, r.v)
				r.c <- struct{}{}
			case r, ok := <-c.rm:
				if !ok {
					return
				}
				cache.remove(r.k)
				r.c <- struct{}{}
			}
		}
	}()

	return c, nil
}

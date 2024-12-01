package lru

import (
	"context"
	"errors"
	"fmt"
	"time"
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

type getResponse struct {
	ok bool
	k  Key
	v  any
}

type getRequest struct {
	k Key
	c chan *getResponse
}

type getLenResponse struct {
	len int
}

type getLenRequest struct {
	c chan *getLenResponse
}

type Cache struct {
	d   time.Duration
	put chan *putRequest
	get chan *getRequest
	rm  chan *removeRequest
	len chan *getLenRequest
}

func (c *Cache) Close() {
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

// Get will retrieve the item with the specified key
// into the cache, updating its lru status.
// An error is raised if the Close() has been called, or
// the timeoout for the operation is exceeded.
func (c *Cache) Get(key Key) (v any, ok bool, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("%v", r)
		}
	}()

	ch := make(chan *getResponse)
	defer close(ch)

	c.get <- &getRequest{
		k: key,
		c: ch,
	}

	select {
	case <-time.After(c.d):
		return nil, false, ErrTimeout
	case r, ok := <-ch:
		if !ok {
			return nil, false, ErrUnknown
		}
		return r.v, r.ok, nil
	}
}

// Len returns the number of items in the cache
// An error is raised if the Close() has been called, or
// the timeoout for the operation is exceeded.
func (c *Cache) Len() (l int, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("%v", r)
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
// the timeoout for the operation is exceeded.
func (c *Cache) Put(key Key, val any) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("%v", r)
		}
	}()

	ch := make(chan struct{})
	defer close(ch)

	c.put <- &putRequest{
		k: key,
		v: val,
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

// Remove will remove the item with the specified key
// from the cache, ignoring if it does not exist.
// An error is raised if the Close() has been called, or
// the timeoout for the operation is exceeded.
func (c *Cache) Remove(key Key) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("%v", r)
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

// New creates a new LRU cache instance with the specified capacity
// and timeout for request processing.
// If capacity > 0 then a new addition will trigger eviction of the
// least recently used item.  If capacity = 0 then cache will grow
// indefinitely.
// If timeout <= 0 then an infinite timeout is used (not recommended)
func New(ctx context.Context, maxEntries int, timeout time.Duration) *Cache {

	if timeout <= 0 {
		timeout = time.Duration(24 * time.Hour) // Effectively infinite
	}

	c := &Cache{
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
				v, ok := cache.get(r.k)
				r.c <- &getResponse{
					ok: ok,
					k:  r.k,
					v:  v,
				}
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

	return c
}

package lru

import "container/list"

// cache is an LRU cache. It is not safe for concurrent access.
type cache struct {
	// capacity is the maximum number of cache entries before
	// an item is evicted. Zero means no limit.
	capacity int

	ll    *list.List
	cache map[interface{}]*list.Element
}

// A Key may be any value that is comparable. See http://golang.org/ref/spec#Comparison_operators
type Key interface{}

type entry struct {
	key   Key
	value interface{}
}

func newCache(maxEntries int) *cache {
	return &cache{
		capacity: maxEntries,
		ll:       list.New(),
		cache:    make(map[interface{}]*list.Element),
	}
}

// put adds a value to the cache.
func (c *cache) put(key Key, value interface{}) {
	if c.cache == nil {
		c.cache = make(map[interface{}]*list.Element)
		c.ll = list.New()
	}
	if ee, ok := c.cache[key]; ok {
		c.ll.MoveToFront(ee)
		ee.Value.(*entry).value = value
		return
	}
	ele := c.ll.PushFront(&entry{key, value})
	c.cache[key] = ele
	if c.capacity != 0 && c.ll.Len() > c.capacity {
		c.removeOldest()
	}
}

// get looks up a key's value from the cache.
func (c *cache) get(key Key) (value interface{}, ok bool) {
	if c.cache == nil {
		return
	}
	if ele, hit := c.cache[key]; hit {
		c.ll.MoveToFront(ele)
		return ele.Value.(*entry).value, true
	}
	return
}

// remove removes the provided key from the cache.
func (c *cache) remove(key Key) {
	if c.cache == nil {
		return
	}
	if ele, hit := c.cache[key]; hit {
		c.removeElement(ele)
	}
}

// removeOldest removes the oldest item from the cache.
func (c *cache) removeOldest() {
	if c.cache == nil {
		return
	}
	ele := c.ll.Back()
	if ele != nil {
		c.removeElement(ele)
	}
}

func (c *cache) removeElement(e *list.Element) {
	c.ll.Remove(e)
	kv := e.Value.(*entry)
	delete(c.cache, kv.key)
}

// len returns the number of items in the cache.
func (c *cache) len() int {
	if c.cache == nil {
		return 0
	}
	return c.ll.Len()
}

// clear purges all stored items from the cache.
func (c *cache) clear() {
	c.ll = nil
	c.cache = nil
}

package lru

import (
	"context"
	"errors"
	"sync"
)

// Partitions mutually divide the cached data
type Partition string

// Partitioner returns the Partition for a given Key, or an error
type Partitioner func(key Key) (Partition, error)

var ErrInvalidPartition = errors.New("partitioner returned unknown partition for key")

// PartitionedCache is an implementation of a Cache that
// splits entries in partitions by their Keys using the
// specified Partitioner function.
// This allows commonly used but slowly changing data to
// avoid eviction and improve responsiveness.
type PartitionedCache struct {
	partitioner Partitioner
	partitions  map[Partition]Cache
	lck         sync.RWMutex
}

func (p *PartitionedCache) getCacheForKey(key Key) (Cache, error) {
	if len(p.partitions) == 0 {
		return nil, ErrAttemptToUseInvalidCache
	}

	part, err := p.partitioner(key)
	if err != nil {
		return nil, err
	}

	p.lck.RLock()
	defer p.lck.RUnlock()

	c, ok := p.partitions[part]
	if !ok {
		return nil, ErrInvalidPartition
	}

	return c, nil
}

// Close empties the cache, releases all resources
func (p *PartitionedCache) Close() {
	p.lck.Lock()
	defer p.lck.Unlock()

	for _, c := range p.partitions {
		c.Close()
	}
	p.partitions = map[Partition]Cache{}
}

// Get retrieves the value at the specified key
func (p *PartitionedCache) Get(key Key) (v any, ok bool, err error) {
	c, err := p.getCacheForKey(key)
	if err != nil {
		return nil, false, err
	}

	return c.Get(key)
}

// Len returns the current usage of the cache
func (p *PartitionedCache) Len() (l int, err error) {
	p.lck.RLock()
	defer p.lck.RUnlock()

	total := 0

	for _, c := range p.partitions {
		l, err := c.Len()
		if err != nil {
			return 0, err
		}
		total += l
	}

	return total, nil
}

// Put inserts the value at the specified key, replacing any prior content
func (p *PartitionedCache) Put(key Key, val any) (err error) {
	c, err := p.getCacheForKey(key)
	if err != nil {
		return err
	}

	return c.Put(key, val)
}

// Remove evicts the key and its associated value
func (p *PartitionedCache) Remove(key Key) (err error) {
	c, err := p.getCacheForKey(key)
	if err != nil {
		return err
	}

	return c.Remove(key)
}

// PartitionInfo specifies the Cache to be used for a given Named partition
type PartitionInfo struct {
	Name  Partition
	Cache Cache
}

var ErrInvalidPartitioner = errors.New("partitioner must not be nil")
var ErrInvalidPartitionInfo = errors.New("caches must not be an empty slice")
var ErrPartitionWithNoCache = errors.New("all partitions must have a non-nil cache")
var ErrPartitionInfoHasDuplicates = errors.New("partitions must have unique names")

// NewPartitionedCache creates a new LRU cache instance consisting of named partitions,
// each of whose data is managed within the provided Cache instance.  The provided Cache
// instances are assumed to be owned by the PartitionedCache instance once they are added.
// Close() should be called when the cache is no longer needed, to release resources.
func NewPartitionedCache(ctx context.Context, partitioner Partitioner, caches []PartitionInfo) (*PartitionedCache, error) {

	if partitioner == nil {
		return nil, ErrInvalidPartitioner
	}

	if len(caches) == 0 {
		return nil, ErrInvalidPartitionInfo
	}

	m := map[Partition]Cache{}
	for _, i := range caches {
		if i.Cache == nil {
			return nil, ErrPartitionWithNoCache
		}
		if _, ok := m[i.Name]; ok {
			return nil, ErrPartitionInfoHasDuplicates
		}
		m[i.Name] = i.Cache
	}

	return &PartitionedCache{
		partitioner: partitioner,
		partitions:  m,
	}, nil
}

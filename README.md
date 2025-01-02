[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](https://en.wikipedia.org/wiki/MIT_License)
[![Documentation](https://img.shields.io/badge/Documentation-GoDoc-green.svg)](https://godoc.org/github.com/gford1000-go/lru)

# LRU | Least Recently Used Cache

Different implementations of the `Cache` interface, all providing different approaches to Least Recently Used caches.

## BasicCache

Implements a concurrency-safe LRU cache, either with finite or unlimited cache capacity.  When finite capacity is 
specified then LRU processing evicts that oldest entry when a new entry is added.

A new cache is created by calling `NewBasicCache` and all cache instances are
independent of each other.

If specified, the timeout value limits the wait time whilst attempting to interact with the cache, and generates an error when the timeout is exceeded.  Setting timeout to zero (infinite wait time on the cache action) avoids the error.

Note the context passed to NewBasicCache() controls the lifetime of the cache as a whole.  This can be different from the context
passed to the Get() which can then control behaviour for each session that is interacting with the cache.

```go
func main() {
    ctx := context.Background()

    cache, _ := NewBasicCache(ctx, 100, 1*time.Millisecond)
    defer cache.Close()

    cache.Put("key", 123) 

    if v, _, _ := cache.Get(ctx, "key"); v != 123 {
        panic("should not happen!")
    }
}
```

If the context is completed in some way, then the cache will be invalidated.

Always call Close() for the cache, to release internal resources (this is automatic if the context completes).

## LoadingCache

This cache extends `BasicCache` to use a `Loader` function to attempt to retrieve and add entries if they are requested
and don't already exist in the cache.  

This simplifies data retrieval logic as it only needs to request entries from the cache, rather than needing additional
code to specify do to load the data (and add to the cache) should the entry be missing from the cache.

If OpenTelemetry is being used, and the context passed to Get() contains a Span, then if the loader is called, events will
be added to that Span to record how many keys are requested and retrieved, together with timestamps.

```go
func main() {
    loader := func(ctx context.Context, key Key) (any, error) {
        // Interact with storage to retrieve
    }

    ctx := context.Background()

    cache, _ := NewLoadingCache(ctx, loader, 100, 1*time.Millisecond)
    defer cache.Close()

    cache.Put("key", 123) 

    if v, _, _ := cache.Get(ctx, "key"); v != 123 {
        panic("should not happen!")
    }
}
```

## PartitionedCache

A partitioned cache is useful when some entries are considered to age more slowly than others; i.e. it is beneficial to retain some of the data in the cache when by normal LRU rules it should be evicted.

The PartitionedCache is simply a facade to any number of `Cache` implementations, that are associated with a specific partition `Name`.

The cache uses a `Partitioner` function that uses the `Key` to resolve which partition (cache) holds the entry, and then cache management of the entry is delegated to the `Cache` implementation of that partition.

This type of cache can for example, avoid reference / static / configuration data being evicted from a `BasicCache` implementation due to other types of data being added to the cache.

```go
func main() {
    ctx := context.Background()

    partitioner := func(key Key) (Partition, error) {
        // Rules specific to Key structure determine partition to return
        return "SomeThing", nil
    }

    someThingsCache, _ := NewBasicCache(ctx, 100, 0)
    otherThingsCache, _ := NewBasicCache(ctx, 500, 0)

    info := []PartitionInfo{
        {
            Name: "SomeThing", 
            Cache: someThingsCache,
        },
        {
            Name: "OtherThings", 
            Cache: otherThingsCache,
        },
    }

    cache, _ := NewPartitionedCache(ctx, partitioner, info)
    defer cache.Close()

    cache.Put("key", 123) 

    if v, _, _ := cache.Get(ctx, "key"); v != 123 {
        panic("should not happen!")
    }
}
```

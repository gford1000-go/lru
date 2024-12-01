
# LRU | Least Recently Used Cache

[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](https://en.wikipedia.org/wiki/MIT_License)
[![Documentation](https://img.shields.io/badge/Documentation-GoDoc-green.svg)](https://godoc.org/github.com/gford1000-go/lru)

A concurrency-safe implementation of a LRU cache.

A new cache is created by calling `New` and all caches instances are
independent of each other.

```go
func main() {
    cache := New(context.Background(), 100, 1*time.Millisecond)
    defer cache.Close()

    cache.Put("key", 123) 

    if v, _, _ := cache.Get("key"); v != 123 {
        panic("should not happen!")
    }
}
```

If the context is completed in some way, then the cache will be invalidated.

Always call Close() for the cache, to release internal resources (this is
automatic if the context completes).

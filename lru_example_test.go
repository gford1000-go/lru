package lru

import (
	"context"
	"fmt"
	"time"
)

func ExampleNewBasicCache() {

	c, _ := NewBasicCache(context.Background(), 10, 1*time.Millisecond)

	// BasicCache implements the Cache interface
	var cache Cache = c

	key := "Key1"
	val := 1234

	cache.Put(key, val) // Add

	v, _, _ := cache.Get(key) // Retrieve

	size1, _ := cache.Len() // Len

	cache.Put(key, val) // Overwrite

	sizeUnchanged, _ := cache.Len() // Has entry

	cache.Remove(key) // Removed

	size0, _ := cache.Len() // Now empty

	_, ok, _ := cache.Get(key) // Not found

	fmt.Println(val == v, size1, sizeUnchanged, size0, ok)
	// Output: true 1 1 0 false
}

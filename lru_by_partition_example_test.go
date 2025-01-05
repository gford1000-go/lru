package lru

import (
	"context"
	"fmt"
)

func ExampleNewPartitionedCache() {

	ctx := context.Background()

	partitioner := func(key Key) (Partition, error) {
		// Rules specific to Key structure determine partition to return
		// Here, just always put into the same partition
		return "SomeThing", nil
	}

	someThingsCache, _ := NewBasicCache(ctx, 100, 0) // Could be LoadingCache or any implementation of Cache
	otherThingsCache, _ := NewBasicCache(ctx, 500, 0)

	info := []PartitionInfo{
		{
			Name:  "SomeThing",
			Cache: someThingsCache,
		},
		{
			Name:  "OtherThings",
			Cache: otherThingsCache,
		},
	}

	cache, _ := NewPartitionedCache(ctx, partitioner, info)
	defer cache.Close()

	err := cache.Put(ctx, "MeaningOfLife", 42)
	if err != nil {
		panic(err)
	}

	v, _, err := cache.Get(ctx, "MeaningOfLife")
	if err != nil {
		panic(err)
	}

	fmt.Println("Meaning of life is still 42?", v.(int) == 42)
	// Output: Meaning of life is still 42? true
}

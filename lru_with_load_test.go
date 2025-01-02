package lru

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestNewLoadingCache(t *testing.T) {
	_, err := NewLoadingCache(context.Background(), nil, 0, 0)

	if err == nil {
		t.Fatal("TestNewLoadingCache fail.  Expected non-nil error")
	}
	if !errors.Is(err, ErrInvalidLoader) {
		t.Fatalf("TestNewLoadingCache fail.  Expected error: %v, got error: %v", ErrInvalidMaxEntries, err)
	}
}

func TestLoadingCache_Get(t *testing.T) {
	loader := func(ctx context.Context, keys []Key) ([]LoaderResult, error) {
		panic("Called!")
	}

	for _, tt := range getTests {
		lru, _ := NewLoadingCache(context.Background(), loader, 0, 0)
		defer lru.Close()

		lru.Put(tt.keyToAdd, 1234)
		val, ok, _ := lru.Get(context.Background(), tt.keyToGet)
		if ok != tt.expectedOk {
			t.Fatalf("TestLoadingCache_Get failed. %s: cache hit = %v; want %v", tt.name, ok, !ok)
		} else if ok && val != 1234 {
			t.Fatalf("TestLoadingCache_Get failed. %s expected get to return 1234 but got %v", tt.name, val)
		}
	}
}

func TestLoadingCache_Remove(t *testing.T) {
	loader := func(ctx context.Context, key []Key) ([]LoaderResult, error) {
		panic("Called!")
	}

	lru, _ := NewLoadingCache(context.Background(), loader, 0, 0)
	defer lru.Close()

	lru.Put("myKey", 1234)
	if val, ok, _ := lru.Get(context.Background(), "myKey"); !ok {
		t.Fatal("TestLoadingCache_Remove returned no match")
	} else if val != 1234 {
		t.Fatalf("TestLoadingCache_Remove failed.  Expected %d, got %v", 1234, val)
	}

	lru.Remove("myKey")
	if _, ok, _ := lru.Get(context.Background(), "myKey"); ok {
		t.Fatal("TestLoadingCache_Remove returned a removed entry")
	}
}

func TestLoadingCache_Len(t *testing.T) {
	loader := func(ctx context.Context, keys []Key) ([]LoaderResult, error) {
		panic("Called!")
	}

	lru, _ := NewLoadingCache(context.Background(), loader, 0, 0)
	defer lru.Close()

	lru.Put("myKey", 1234)
	if val, _ := lru.Len(); val != 1 {
		t.Fatalf("TestLoadingCache_Len failed.  Expected %d, got %v", 1234, val)
	}

	lru.Remove("myKey")
	if val, _ := lru.Len(); val != 0 {
		t.Fatalf("TestLoadingCache_Len failed.  Expected %d, got %v", 1234, val)
	}
}

func TestLoadingCache_Get_1(t *testing.T) {
	loader := func(ctx context.Context, keys []Key) ([]LoaderResult, error) {
		panic("Called!")
	}

	lru, _ := NewLoadingCache(context.Background(), loader, 0, 0)
	defer lru.Close()

	v, ok, err := lru.Get(context.Background(), "Failure")
	if err == nil {
		t.Fatal("TestLoadingCache_Get_1 failed.  Expected an error, got nil")
	}

	if err.Error() != "unexpected error: Called!" {
		t.Fatalf("TestLoadingCache_Get_1 failed.  Expected error 'unexpected error: Called!', got '%v'", err.Error())
	}

	if v != nil {
		t.Fatal("TestLoadingCache_Get_1 failed.  Expected an nil value, got non-nil")
	}

	if ok {
		t.Fatal("TestLoadingCache_Get_1 failed.  Expected ok = false, got ok = true")
	}
}

func TestLoadingCache_Get_2(t *testing.T) {

	var v *int = new(int)
	var meaning = 42

	loader := func(ctx context.Context, keys []Key) ([]LoaderResult, error) {
		(*v) += meaning
		return []LoaderResult{
			{
				Key:   "Meaning of Life",
				Value: *v,
			},
		}, nil
	}

	lru, _ := NewLoadingCache(context.Background(), loader, 0, 0)
	defer lru.Close()

	f := func() {

		v, ok, err := lru.Get(context.Background(), "Meaning of Life")
		if err != nil {
			t.Fatalf("TestLoadingCache_Get_2 failed.  Expected no error, got '%v'", err)
		}

		if v.(int) != meaning {
			t.Fatalf("TestLoadingCache_Get_2 failed.  Expected value = %v, got %v", meaning, v.(int))
		}

		if !ok {
			t.Fatal("TestLoadingCache_Get_2 failed.  Expected ok = true, got ok = false")
		}

		time.Sleep(100 * time.Microsecond) // Allow LoadingCache to populate on separate goroutine

		l, err := lru.Len()
		if err != nil {
			t.Fatalf("TestLoadingCache_Get_2 failed.  Expected no error, got '%v'", err)
		}

		if l != 1 {
			t.Fatalf("TestLoadingCache_Get_2 failed.  Expected Len = 1, got %v", l)
		}

	}

	f() // first test verify can insert
	f() // second test verifies retrieved from cache, with no reinsert
}

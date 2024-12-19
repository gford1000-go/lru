/*
Copyright 2013 Google Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package lru

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
)

type simpleStruct struct {
	int
	string
}

type complexStruct struct {
	int
	simpleStruct
}

var getTests = []struct {
	name       string
	keyToAdd   interface{}
	keyToGet   interface{}
	expectedOk bool
}{
	{"string_hit", "myKey", "myKey", true},
	{"string_miss", "myKey", "nonsense", false},
	{"simple_struct_hit", simpleStruct{1, "two"}, simpleStruct{1, "two"}, true},
	{"simple_struct_miss", simpleStruct{1, "two"}, simpleStruct{0, "noway"}, false},
	{"complex_struct_hit", complexStruct{1, simpleStruct{2, "three"}},
		complexStruct{1, simpleStruct{2, "three"}}, true},
}

func TestBasicCache_Get(t *testing.T) {
	for _, tt := range getTests {
		lru, _ := NewBasicCache(context.Background(), 0, 0)
		defer lru.Close()

		lru.Put(tt.keyToAdd, 1234)
		val, ok, _ := lru.Get(tt.keyToGet)
		if ok != tt.expectedOk {
			t.Fatalf("TestBasicCache_Get failed.  %s: cache hit = %v; want %v", tt.name, ok, !ok)
		} else if ok && val != 1234 {
			t.Fatalf("TestBasicCache_Get failed.  %s expected get to return 1234 but got %v", tt.name, val)
		}
	}
}

func TestBasicCache_Remove(t *testing.T) {
	lru, _ := NewBasicCache(context.Background(), 0, 0)
	defer lru.Close()

	lru.Put("myKey", 1234)
	if val, ok, _ := lru.Get("myKey"); !ok {
		t.Fatal("TestBasicCache_Remove returned no match")
	} else if val != 1234 {
		t.Fatalf("TestBasicCache_Remove failed.  Expected %d, got %v", 1234, val)
	}

	lru.Remove("myKey")
	if _, ok, _ := lru.Get("myKey"); ok {
		t.Fatal("TestBasicCache_Remove returned a removed entry")
	}
}

func TestBasicCache_Len(t *testing.T) {
	lru, _ := NewBasicCache(context.Background(), 0, 0)
	defer lru.Close()

	lru.Put("myKey", 1234)
	if val, _ := lru.Len(); val != 1 {
		t.Fatalf("TestBasicCache_Len failed.  Expected %d, got %v", 1, val)
	}

	lru.Remove("myKey")
	if val, _ := lru.Len(); val != 0 {
		t.Fatalf("TestBasicCache_Len failed.  Expected %d, got %v", 0, val)
	}
}

func TestBasicCache_Put_1(t *testing.T) {
	lru, _ := NewBasicCache(context.Background(), 0, 0)
	defer lru.Close()

	for i := 0; i < 10; i++ {
		if err := lru.Put("myKey", i); err != nil {
			t.Errorf("TestBasicCache_Put_1 failed.  Expected success, but got error %v", err)
		}
	}

	if val, _ := lru.Len(); val != 1 {
		t.Fatalf("TestBasicCache_Put_1 failed.  Expected %d, got %v", 1, val)
	}

	val, ok, err := lru.Get("myKey")
	if err != nil {
		t.Errorf("TestBasicCache_Put_1 failed.  Expected success, but got error %v", err)
	}
	if !ok {
		t.Error("TestBasicCache_Put_1 failed.  Expected ok = true, but got false")
	}
	if val.(int) != 9 {
		t.Fatalf("TestBasicCache_Put_1 failed.  Expected v = 9, got %v", val)
	}
}

func TestBasicCache_Put_2(t *testing.T) {

	lru, _ := NewBasicCache(context.Background(), 0, 0)
	defer lru.Close()

	var n int = 10000

	var c chan error = make(chan error, n)
	defer close(c)

	make_key := func(i int) Key { return fmt.Sprintf("Key_%d", i) }

	// Concurrent puts to the cache
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(v int) {
			defer wg.Done()
			c <- lru.Put(make_key(v), v)
		}(i)
	}
	wg.Wait()

	for i := 0; i < n; i++ {
		e := <-c
		if e != nil {
			t.Errorf("TestBasicCache_Put_2 failed.  Expected success, but got error %v", e)
		}
	}

	if val, _ := lru.Len(); val != n {
		t.Fatalf("TestBasicCache_Put_2 failed.  Expected %d, got %v", n, val)
	}

	for i := 0; i < n; i++ {
		val, ok, err := lru.Get(make_key(i))
		if err != nil {
			t.Fatalf("TestBasicCache_Put_1 failed.  Expected success, but got error %v", err)
		}
		if !ok {
			t.Fatalf("TestBasicCache_Put_1 failed.  Expected ok = true, but got false")
		}
		if val.(int) != i {
			t.Fatalf("TestBasicCache_Put_1 failed.  Expected v = %v, got %v", i, val)
		}
	}
}

func TestBasicCache_Put_3(t *testing.T) {

	maxSize := 1000

	lru, _ := NewBasicCache(context.Background(), maxSize, 0)
	defer lru.Close()

	var n int = maxSize * 2 // Should start evicting to maintain maxSize

	var c chan error = make(chan error, n)
	defer close(c)

	make_key := func(i int) Key { return fmt.Sprintf("Key_%d", i) }

	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(v int) {
			defer wg.Done()
			c <- lru.Put(make_key(v), v)
		}(i)
	}
	wg.Wait()

	for i := 0; i < n; i++ {
		e := <-c
		if e != nil {
			t.Fatalf("TestBasicCache_Put_3 failed.  Expected success, but got error %v", e)
		}
	}

	if val, _ := lru.Len(); val != maxSize {
		t.Fatalf("TestBasicCache_Put_3 failed.  Expected %d, got %v", maxSize, val)
	}
}

func TestBasicCache_Close(t *testing.T) {
	lru, _ := NewBasicCache(context.Background(), 0, 0)

	// Calling Close() more than once is harmless
	lru.Close()
	lru.Close()
}

func TestBasicCache_Close_1(t *testing.T) {
	lru, _ := NewBasicCache(context.Background(), 0, 0)

	// Calling lru after Close() generates error
	lru.Close()

	len, err := lru.Len()
	if err == nil {
		t.Fatal("TestBasicCache_Close_1 fail.  Expected non-nil error")
	}
	if !errors.Is(err, ErrAttemptToUseInvalidCache) {
		t.Fatalf("TestBasicCache_Close_1 fail.  Expected error: %v, got error: %v", ErrAttemptToUseInvalidCache, err)
	}
	if len != 0 {
		t.Fatalf("TestBasicCache_Close_1 fail.  Expected Len = 0, but got %v", len)
	}
}

func TestNewBasicCache(t *testing.T) {
	_, err := NewBasicCache(context.Background(), -1, 0)

	if err == nil {
		t.Fatal("TestNewBasicCache fail.  Expected non-nil error")
	}
	if !errors.Is(err, ErrInvalidMaxEntries) {
		t.Fatalf("TestNewBasicCache fail.  Expected error: %v, got error: %v", ErrInvalidMaxEntries, err)
	}
}

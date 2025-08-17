package main

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewLRUCache(t *testing.T) {
	tests := []struct {
		name     string
		capacity int
		expected int
	}{
		{"positive capacity", 10, 10},
		{"small capacity", 1, 1},
		{"large capacity", 1000, 1000},
		{"zero capacity", 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cache := NewLRUCache(tt.capacity)
			require.NotNil(t, cache)
			assert.Equal(t, tt.expected, cache.capacity)
			assert.Equal(t, 0, cache.Size())
		})
	}
}

func TestLRUCache_PutAndGet(t *testing.T) {
	tests := []struct {
		name     string
		capacity int
		ops      []struct {
			action   string
			key      string
			item     CacheItem
			expected CacheItem
			found    bool
		}
	}{
		{
			name:     "single item put and get",
			capacity: 2,
			ops: []struct {
				action   string
				key      string
				item     CacheItem
				expected CacheItem
				found    bool
			}{
				{"put", "key1", CacheItem{ID: "id1", Content: "content1"}, CacheItem{}, false},
				{"get", "key1", CacheItem{}, CacheItem{ID: "id1", Content: "content1"}, true},
			},
		},
		{
			name:     "multiple items within capacity",
			capacity: 3,
			ops: []struct {
				action   string
				key      string
				item     CacheItem
				expected CacheItem
				found    bool
			}{
				{"put", "key1", CacheItem{ID: "id1", Content: "content1"}, CacheItem{}, false},
				{"put", "key2", CacheItem{ID: "id2", Content: "content2"}, CacheItem{}, false},
				{"get", "key1", CacheItem{}, CacheItem{ID: "id1", Content: "content1"}, true},
				{"get", "key2", CacheItem{}, CacheItem{ID: "id2", Content: "content2"}, true},
				{"get", "nonexistent", CacheItem{}, CacheItem{}, false},
			},
		},
		{
			name:     "update existing item",
			capacity: 2,
			ops: []struct {
				action   string
				key      string
				item     CacheItem
				expected CacheItem
				found    bool
			}{
				{"put", "key1", CacheItem{ID: "id1", Content: "content1"}, CacheItem{}, false},
				{"put", "key1", CacheItem{ID: "id1", Content: "updated_content"}, CacheItem{}, false},
				{"get", "key1", CacheItem{}, CacheItem{ID: "id1", Content: "updated_content"}, true},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cache := NewLRUCache(tt.capacity)

			for _, op := range tt.ops {
				switch op.action {
				case "put":
					cache.Put(op.key, op.item)
				case "get":
					item, found := cache.Get(op.key)
					assert.Equal(t, op.found, found)
					if found {
						assert.Equal(t, op.expected.ID, item.ID)
						assert.Equal(t, op.expected.Content, item.Content)
						assert.WithinDuration(t, time.Now(), item.LastUsed, time.Second)
					}
				}
			}
		})
	}
}

func TestLRUCache_EvictionBehavior(t *testing.T) {
	tests := []struct {
		name     string
		capacity int
		sequence []struct {
			action string
			key    string
			item   CacheItem
		}
		finalChecks []struct {
			key   string
			found bool
		}
	}{
		{
			name:     "evict least recently used",
			capacity: 2,
			sequence: []struct {
				action string
				key    string
				item   CacheItem
			}{
				{"put", "key1", CacheItem{ID: "id1", Content: "content1"}},
				{"put", "key2", CacheItem{ID: "id2", Content: "content2"}},
				{"put", "key3", CacheItem{ID: "id3", Content: "content3"}},
			},
			finalChecks: []struct {
				key   string
				found bool
			}{
				{"key1", false},
				{"key2", true},
				{"key3", true},
			},
		},
		{
			name:     "get updates usage order",
			capacity: 2,
			sequence: []struct {
				action string
				key    string
				item   CacheItem
			}{
				{"put", "key1", CacheItem{ID: "id1", Content: "content1"}},
				{"put", "key2", CacheItem{ID: "id2", Content: "content2"}},
				{"get", "key1", CacheItem{}},
				{"put", "key3", CacheItem{ID: "id3", Content: "content3"}},
			},
			finalChecks: []struct {
				key   string
				found bool
			}{
				{"key1", true},
				{"key2", false},
				{"key3", true},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cache := NewLRUCache(tt.capacity)

			for _, op := range tt.sequence {
				switch op.action {
				case "put":
					cache.Put(op.key, op.item)
				case "get":
					cache.Get(op.key)
				}
			}

			for _, check := range tt.finalChecks {
				_, found := cache.Get(check.key)
				assert.Equal(t, check.found, found, "key %s should have found=%v", check.key, check.found)
			}

			assert.Equal(t, tt.capacity, cache.Size())
		})
	}
}

func TestLRUCache_ConcurrentAccess(t *testing.T) {
	cache := NewLRUCache(100)
	var wg sync.WaitGroup
	numGoroutines := 10
	numOperations := 100

	wg.Add(numGoroutines * 2)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				key := "key" + string(rune(id*numOperations+j))
				item := CacheItem{
					ID:      "id" + string(rune(id*numOperations+j)),
					Content: "content" + string(rune(id*numOperations+j)),
				}
				cache.Put(key, item)
			}
		}(i)

		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				key := "key" + string(rune(id*numOperations+j))
				cache.Get(key)
			}
		}(i)
	}

	wg.Wait()

	assert.LessOrEqual(t, cache.Size(), 100)
}

func TestLRUCache_EdgeCases(t *testing.T) {
	tests := []struct {
		name string
		test func(t *testing.T)
	}{
		{
			name: "zero capacity cache",
			test: func(t *testing.T) {
				cache := NewLRUCache(0)
				cache.Put("key1", CacheItem{ID: "id1", Content: "content1"})
				assert.Equal(t, 0, cache.Size())
				_, found := cache.Get("key1")
				assert.False(t, found)
			},
		},
		{
			name: "single capacity cache",
			test: func(t *testing.T) {
				cache := NewLRUCache(1)
				cache.Put("key1", CacheItem{ID: "id1", Content: "content1"})
				assert.Equal(t, 1, cache.Size())

				cache.Put("key2", CacheItem{ID: "id2", Content: "content2"})
				assert.Equal(t, 1, cache.Size())

				_, found := cache.Get("key1")
				assert.False(t, found)

				_, found = cache.Get("key2")
				assert.True(t, found)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, tt.test)
	}
}

func TestLRUCache_UtilityMethods(t *testing.T) {
	tests := []struct {
		name     string
		capacity int
		setup    func(*LRUCache)
		test     func(*testing.T, *LRUCache)
	}{
		{
			name:     "size method accuracy",
			capacity: 5,
			setup: func(c *LRUCache) {
				for i := 0; i < 3; i++ {
					c.Put("key"+string(rune(i)), CacheItem{ID: "id" + string(rune(i))})
				}
			},
			test: func(t *testing.T, c *LRUCache) {
				assert.Equal(t, 3, c.Size())
			},
		},
		{
			name:     "clear method",
			capacity: 3,
			setup: func(c *LRUCache) {
				for i := 0; i < 3; i++ {
					c.Put("key"+string(rune(i)), CacheItem{ID: "id" + string(rune(i))})
				}
			},
			test: func(t *testing.T, c *LRUCache) {
				assert.Equal(t, 3, c.Size())
				c.Clear()
				assert.Equal(t, 0, c.Size())
				for i := 0; i < 3; i++ {
					_, found := c.Get("key" + string(rune(i)))
					assert.False(t, found)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cache := NewLRUCache(tt.capacity)
			tt.setup(cache)
			tt.test(t, cache)
		})
	}
}

func TestLRUCache_LastUsedTimestamp(t *testing.T) {
	cache := NewLRUCache(2)
	startTime := time.Now()

	item := CacheItem{ID: "id1", Content: "content1"}
	cache.Put("key1", item)

	retrieved, found := cache.Get("key1")
	require.True(t, found)
	assert.True(t, retrieved.LastUsed.After(startTime))
	assert.WithinDuration(t, time.Now(), retrieved.LastUsed, time.Second)

	time.Sleep(10 * time.Millisecond)
	firstAccess := retrieved.LastUsed

	retrieved2, found := cache.Get("key1")
	require.True(t, found)
	assert.True(t, retrieved2.LastUsed.After(firstAccess))
}
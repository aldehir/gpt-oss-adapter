package main

import (
	"container/list"
	"sync"
)

type ReasoningItem struct {
	ID      string
	Content string
}

type LRUCache struct {
	capacity int
	cache    map[string]*list.Element
	list     *list.List
	mutex    sync.RWMutex
}

type cacheEntry struct {
	key  string
	item ReasoningItem
}

func NewLRUCache(capacity int) *LRUCache {
	return &LRUCache{
		capacity: capacity,
		cache:    make(map[string]*list.Element),
		list:     list.New(),
	}
}

func (c *LRUCache) Get(key string) (ReasoningItem, bool) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if elem, exists := c.cache[key]; exists {
		c.list.MoveToFront(elem)
		entry := elem.Value.(*cacheEntry)
		return entry.item, true
	}
	return ReasoningItem{}, false
}

func (c *LRUCache) Put(key string, item ReasoningItem) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.capacity <= 0 {
		return
	}

	if elem, exists := c.cache[key]; exists {
		c.list.MoveToFront(elem)
		entry := elem.Value.(*cacheEntry)
		entry.item = item
		return
	}

	if c.list.Len() >= c.capacity {
		c.evictLRU()
	}

	entry := &cacheEntry{key: key, item: item}
	elem := c.list.PushFront(entry)
	c.cache[key] = elem
}

func (c *LRUCache) evictLRU() {
	elem := c.list.Back()
	if elem != nil {
		c.list.Remove(elem)
		entry := elem.Value.(*cacheEntry)
		delete(c.cache, entry.key)
	}
}

func (c *LRUCache) Size() int {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return c.list.Len()
}

func (c *LRUCache) Clear() {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.cache = make(map[string]*list.Element)
	c.list = list.New()
}

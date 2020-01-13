package cache

import (
)

type Cache interface {
	Get(key string) interface{}
	Put(key string, obj interface{})
}

// A simple map cache is a cache that allows the caller to store a single object per key. The entire set
// of keyed entries is controlled by a single global lock. The key can be empty string and the cached object
// can be nil. It is up to the caller to be smart.
func NewSimpleMapCache() Cache {
	return new(SimpleMapCache)
}
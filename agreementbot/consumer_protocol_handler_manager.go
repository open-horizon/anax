package agreementbot

import (
	"sync"
)

// An object to control concurrent access to the map of all consumer protocol handlers. CPHs are never removed. This allows the
// Has() function to be used before the Add() function without holding the lock across both function calls.
type ConsumerPHMgr struct {
	mapLock sync.Mutex                         // Lock protecting access to the map
	cphMap  map[string]ConsumerProtocolHandler // The set of CPHs (one per agreement protocol) in use by policies and patterns in the system.
}

func NewConsumerPHMgr() *ConsumerPHMgr {
	return &ConsumerPHMgr{
		cphMap: make(map[string]ConsumerProtocolHandler),
	}
}

// Returns true when the input protocol already has a CPH in place.
func (c *ConsumerPHMgr) Has(protocol string) bool {
	c.mapLock.Lock()
	defer c.mapLock.Unlock()
	_, ok := c.cphMap[protocol]
	return ok
}

// Unconditionally add the given CPH.
func (c *ConsumerPHMgr) Add(protocol string, cph ConsumerProtocolHandler) {
	c.mapLock.Lock()
	defer c.mapLock.Unlock()
	c.cphMap[protocol] = cph
}

// Retrieve the given CPH.
func (c *ConsumerPHMgr) Get(protocol string) ConsumerProtocolHandler {
	c.mapLock.Lock()
	defer c.mapLock.Unlock()
	return c.cphMap[protocol]
}

// Iterate over each CPH.
func (c *ConsumerPHMgr) GetAll() []string {
	c.mapLock.Lock()
	defer c.mapLock.Unlock()
	keys := make([]string, 0, len(c.cphMap))
	for k := range c.cphMap {
		keys = append(keys, k)
	}
	return keys
}

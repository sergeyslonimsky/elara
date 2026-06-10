package schemavalidator

import (
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

type cacheKey struct {
	namespace   string
	pathPattern string
	schemaHash  string
}

type compiledSchemaCache struct {
	mu      sync.RWMutex
	entries map[cacheKey]*jsonschema.Schema
}

func newCompiledSchemaCache() *compiledSchemaCache {
	return &compiledSchemaCache{entries: make(map[cacheKey]*jsonschema.Schema)}
}

func (c *compiledSchemaCache) get(k cacheKey) (*jsonschema.Schema, bool) {
	c.mu.RLock()

	defer c.mu.RUnlock()

	s, ok := c.entries[k]

	return s, ok
}

func (c *compiledSchemaCache) set(k cacheKey, s *jsonschema.Schema) {
	c.mu.Lock()

	defer c.mu.Unlock()

	c.entries[k] = s
}

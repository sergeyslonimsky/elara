package schemavalidator

import (
	"sync"

	"github.com/gobwas/glob"
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

// compiledPattern is a path pattern's compiled glob plus its precomputed
// specificity score.
//
// err is kept rather than dropped so that an invalid pattern is remembered as
// invalid: without it, a malformed attachment would be recompiled — and fail —
// on every single write.
type compiledPattern struct {
	glob        glob.Glob
	specificity int
	err         error
}

type compiledPatternCache struct {
	mu      sync.RWMutex
	entries map[string]compiledPattern
}

func newCompiledPatternCache() *compiledPatternCache {
	return &compiledPatternCache{entries: make(map[string]compiledPattern)}
}

// compile returns the pattern's compiled form, compiling it on first use.
//
// The key space is bounded by the path patterns operators have actually
// attached, so this grows with configuration rather than with traffic.
//
// Compilation deliberately happens outside the lock. Two writers racing on the
// same new pattern will each compile it and the last store wins, which costs
// one redundant compile; holding the write lock across compilation would
// instead serialise every validating write behind it.
func (c *compiledPatternCache) compile(pattern string) compiledPattern {
	c.mu.RLock()
	entry, ok := c.entries[pattern]
	c.mu.RUnlock()

	if ok {
		return entry
	}

	g, err := glob.Compile(pattern, '/')
	entry = compiledPattern{glob: g, specificity: specificity(pattern), err: err}

	c.mu.Lock()
	c.entries[pattern] = entry
	c.mu.Unlock()

	return entry
}

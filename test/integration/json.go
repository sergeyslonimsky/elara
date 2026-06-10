package integration

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// SentinelPresence in a resp.json value asserts the field must exist and be non-empty.
const SentinelPresence = "<<PRESENCE>>"

// CompareJSON recursively compares expected vs actual JSON values.
// SentinelPresence in expected asserts the corresponding actual field is non-empty.
func CompareJSON(t *testing.T, path string, expected, actual any) {
	t.Helper()

	switch e := expected.(type) {
	case string:
		if e == SentinelPresence {
			assert.NotEmpty(t, actual, "%s: expected presence, got empty/nil", path)

			return
		}

		assert.Equal(t, e, actual, path)

	case map[string]any:
		aMap, ok := actual.(map[string]any)
		if !assert.True(t, ok, "%s: expected object, got %T", path, actual) {
			return
		}

		for k, v := range e {
			CompareJSON(t, path+"."+k, v, aMap[k])
		}

	case []any:
		aArr, ok := actual.([]any)
		if !assert.True(t, ok, "%s: expected array, got %T", path, actual) {
			return
		}

		if !assert.Len(t, aArr, len(e), "%s: array length mismatch", path) {
			return
		}

		for i := range e {
			CompareJSON(t, fmt.Sprintf("%s[%d]", path, i), e[i], aArr[i])
		}

	default:
		assert.Equal(t, expected, actual, path)
	}
}

// ReadJSON reads the JSON file at path and unmarshals it into v.
func ReadJSON(t *testing.T, path string, v any) {
	t.Helper()
	require.NoError(t, json.Unmarshal(ReadFile(t, path), v))
}

// CompareJSONBytes parses expected and actual as JSON and delegates to CompareJSON.
// Use this when both sides are raw []byte (e.g. a golden file and an HTTP response body).
func CompareJSONBytes(t *testing.T, expected, actual []byte) {
	t.Helper()

	var e, a any
	require.NoError(t, json.Unmarshal(expected, &e), "parsing expected JSON")
	require.NoError(t, json.Unmarshal(actual, &a), "parsing actual JSON")

	CompareJSON(t, "$", e, a)
}

// ReadFile reads and returns the raw bytes of the file at path.
func ReadFile(t *testing.T, path string) []byte {
	t.Helper()

	b, err := os.ReadFile(path)
	require.NoError(t, err, "reading testdata file: %s", path)

	return b
}

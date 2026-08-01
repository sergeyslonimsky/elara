package bbolt_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/pkg/bbolt"
)

func TestJSONCodec_Marshal_Success(t *testing.T) {
	t.Parallel()

	c := bbolt.JSONCodec[testItem]{}

	data, err := c.Marshal(testItem{ID: "a", Value: 1})
	require.NoError(t, err)
	assert.JSONEq(t, `{"id":"a","value":1}`, string(data))
}

func TestJSONCodec_Marshal_Error(t *testing.T) {
	t.Parallel()

	c := bbolt.JSONCodec[chan int]{}

	_, err := c.Marshal(make(chan int))
	require.ErrorContains(t, err, "json marshal:")
}

func TestJSONCodec_Unmarshal_Success(t *testing.T) {
	t.Parallel()

	c := bbolt.JSONCodec[testItem]{}

	var out testItem

	err := c.Unmarshal([]byte(`{"id":"a","value":1}`), &out)
	require.NoError(t, err)
	assert.Equal(t, testItem{ID: "a", Value: 1}, out)
}

func TestJSONCodec_Unmarshal_Error(t *testing.T) {
	t.Parallel()

	c := bbolt.JSONCodec[testItem]{}

	var out testItem

	err := c.Unmarshal([]byte("not-json"), &out)
	require.ErrorContains(t, err, "json unmarshal:")
}

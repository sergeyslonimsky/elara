package config

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

// Compare semantics are etcd's, and they moved here from handler/etcdv3 with the
// Txn orchestration. An unknown target or relation must evaluate to false rather
// than error: the handler maps an unrecognised proto enum to 0, and the
// pre-existing behaviour on that input was "the condition does not hold, take
// the failure branch", not "reject the request".

func TestCompareInt64(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		op        KVCompareResult
		got, want int64
		expected  bool
	}{
		{name: "equal holds", op: KVCompareEqual, got: 5, want: 5, expected: true},
		{name: "equal fails", op: KVCompareEqual, got: 5, want: 6, expected: false},
		{name: "not equal holds", op: KVCompareNotEqual, got: 5, want: 6, expected: true},
		{name: "not equal fails", op: KVCompareNotEqual, got: 5, want: 5, expected: false},
		{name: "greater holds", op: KVCompareGreater, got: 6, want: 5, expected: true},
		{name: "greater is strict", op: KVCompareGreater, got: 5, want: 5, expected: false},
		{name: "greater fails", op: KVCompareGreater, got: 4, want: 5, expected: false},
		{name: "less holds", op: KVCompareLess, got: 4, want: 5, expected: true},
		{name: "less is strict", op: KVCompareLess, got: 5, want: 5, expected: false},
		{name: "unknown relation is false", op: 0, got: 5, want: 5, expected: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.expected, compareInt64(tt.op, tt.got, tt.want))
		})
	}
}

func TestCompareBytes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		op        KVCompareResult
		got, want []byte
		expected  bool
	}{
		{
			name: "equal holds",
			op:   KVCompareEqual, got: []byte("abc"), want: []byte("abc"), expected: true,
		},
		{
			name: "equal fails",
			op:   KVCompareEqual, got: []byte("abc"), want: []byte("abd"), expected: false,
		},
		{name: "nil equals nil", op: KVCompareEqual, got: nil, want: nil, expected: true},
		{
			name: "empty equals nil",
			op:   KVCompareEqual, got: []byte{}, want: nil, expected: true,
		},
		{
			name: "not equal holds",
			op:   KVCompareNotEqual, got: []byte("abc"), want: []byte("abd"), expected: true,
		},
		{
			name: "greater holds",
			op:   KVCompareGreater, got: []byte("b"), want: []byte("a"), expected: true,
		},
		{
			name: "greater fails",
			op:   KVCompareGreater, got: []byte("a"), want: []byte("b"), expected: false,
		},
		{
			name: "less holds",
			op:   KVCompareLess, got: []byte("a"), want: []byte("b"), expected: true,
		},
		{
			name: "unknown relation is false",
			op:   0, got: []byte("a"), want: []byte("a"), expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.expected, compareBytes(tt.op, tt.got, tt.want))
		})
	}
}

func TestCompareKV(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		cmp      KVCompare
		kv       *domain.KVPair
		expected bool
	}{
		{
			name: "version equal",
			cmp: KVCompare{
				Target: KVCompareVersion, Result: KVCompareEqual, WantRevision: 5,
			},
			kv:       &domain.KVPair{Version: 5},
			expected: true,
		},
		{
			name: "version mismatch",
			cmp: KVCompare{
				Target: KVCompareVersion, Result: KVCompareEqual, WantRevision: 6,
			},
			kv:       &domain.KVPair{Version: 5},
			expected: false,
		},
		{
			name: "create revision greater",
			cmp: KVCompare{
				Target: KVCompareCreateRevision, Result: KVCompareGreater, WantRevision: 5,
			},
			kv:       &domain.KVPair{CreateRevision: 10},
			expected: true,
		},
		{
			name: "mod revision less",
			cmp: KVCompare{
				Target: KVCompareModRevision, Result: KVCompareLess, WantRevision: 30,
			},
			kv:       &domain.KVPair{ModRevision: 20},
			expected: true,
		},
		{
			name: "value equal",
			cmp: KVCompare{
				Target: KVCompareValue, Result: KVCompareEqual, WantValue: []byte("hello"),
			},
			kv:       &domain.KVPair{Value: []byte("hello")},
			expected: true,
		},
		{
			name: "value mismatch",
			cmp: KVCompare{
				Target: KVCompareValue, Result: KVCompareEqual, WantValue: []byte("world"),
			},
			kv:       &domain.KVPair{Value: []byte("hello")},
			expected: false,
		},
		{
			// The create-if-absent lock recipe: a missing key has
			// CreateRevision 0, so Compare(CreateRevision, =, 0) holds.
			name: "absent key has create revision 0",
			cmp: KVCompare{
				Target: KVCompareCreateRevision, Result: KVCompareEqual, WantRevision: 0,
			},
			kv:       nil,
			expected: true,
		},
		{
			name: "absent key has version 0",
			cmp: KVCompare{
				Target: KVCompareVersion, Result: KVCompareEqual, WantRevision: 0,
			},
			kv:       nil,
			expected: true,
		},
		{
			name: "absent key has nil value",
			cmp: KVCompare{
				Target: KVCompareValue, Result: KVCompareEqual, WantValue: nil,
			},
			kv:       nil,
			expected: true,
		},
		{
			name:     "unknown target is false",
			cmp:      KVCompare{Target: 0, Result: KVCompareEqual},
			kv:       &domain.KVPair{},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.expected, compareKV(tt.cmp, tt.kv))
		})
	}
}

func TestKVTxnInput_hasWrites(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		in       KVTxnInput
		expected bool
	}{
		{name: "empty", in: KVTxnInput{}, expected: false},
		{
			name: "range only in both branches",
			in: KVTxnInput{
				Success: []KVOp{{Kind: KVOpRange, Range: &KVRangeOp{}}},
				Failure: []KVOp{{Kind: KVOpRange, Range: &KVRangeOp{}}},
			},
			expected: false,
		},
		{
			name: "put in the success branch",
			in: KVTxnInput{
				Success: []KVOp{{Kind: KVOpPut, Put: &KVPutOp{}}},
			},
			expected: true,
		},
		{
			// Conservative on purpose: the branch is unknown until the compares
			// run, and they run inside the boundary this decides to open.
			name: "delete in the branch that may not run",
			in: KVTxnInput{
				Success: []KVOp{{Kind: KVOpRange, Range: &KVRangeOp{}}},
				Failure: []KVOp{{Kind: KVOpDeleteRange, DeleteRange: &KVDeleteRangeOp{}}},
			},
			expected: true,
		},
		{
			name: "write buried in a nested transaction",
			in: KVTxnInput{
				Success: []KVOp{{
					Kind: KVOpTxn,
					Txn: &KVTxnInput{
						Failure: []KVOp{{Kind: KVOpPut, Put: &KVPutOp{}}},
					},
				}},
			},
			expected: true,
		},
		{
			name: "read-only nested transaction",
			in: KVTxnInput{
				Success: []KVOp{{
					Kind: KVOpTxn,
					Txn: &KVTxnInput{
						Success: []KVOp{{Kind: KVOpRange, Range: &KVRangeOp{}}},
					},
				}},
			},
			expected: false,
		},
		{
			name:     "nil nested transaction",
			in:       KVTxnInput{Success: []KVOp{{Kind: KVOpTxn}}},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.expected, tt.in.hasWrites())
		})
	}
}

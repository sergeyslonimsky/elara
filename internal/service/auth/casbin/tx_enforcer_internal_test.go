package casbin

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/internal/service/adapter/bbolt"
	"github.com/sergeyslonimsky/elara/internal/service/storage"
)

// newInternalTestEnforcer creates an Enforcer and TxManager backed by a real
// bbolt store in t.TempDir. Returned together so tests can drive WriteTx /
// WithTx via the real tx infrastructure.
func newInternalTestEnforcer(t *testing.T) (*Enforcer, *bbolt.TxManager, *bbolt.PolicyRepo) {
	t.Helper()

	path := filepath.Join(t.TempDir(), "casbin.db")

	store, err := bbolt.Open(path)
	require.NoError(t, err)

	t.Cleanup(func() { _ = store.Close() })

	policies := bbolt.NewPolicyRepo(store)

	e, err := NewEnforcer(policies)
	require.NoError(t, err)

	txm := bbolt.NewTxManager(store.DB())

	return e, txm, policies
}

// cachePolicyContains reports whether the in-memory enforcer cache contains a
// p-rule equal to want.
func cachePolicyContains(rules [][]string, want []string) bool {
	for _, r := range rules {
		if equalRule(r, want) {
			return true
		}
	}

	return false
}

func equalRule(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}

	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}

	return true
}

func TestTxEnforcer_AddPolicy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		rule    []string
		wantOp  opKind
		wantErr string
	}{
		{
			name:   "records op and persists rule",
			rule:   []string{"role:custom", "*", "config", "read"},
			wantOp: opAddP,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			e, txm, _ := newInternalTestEnforcer(t)

			var captured *TxEnforcer

			err := txm.Write(t.Context(), func(tx storage.Tx) error {
				txe := e.WithTx(tx)
				captured = txe

				return txe.AddPolicy(tt.rule[0], tt.rule[1], tt.rule[2], tt.rule[3])
			})

			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)

				return
			}
			require.NoError(t, err)
			require.NotNil(t, captured)

			// Op recorded with correct kind and args.
			require.Len(t, captured.ops, 1)
			assert.Equal(t, tt.wantOp, captured.ops[0].kind)
			assert.Equal(t, tt.rule, captured.ops[0].args)

			// Cache NOT updated by TxEnforcer alone.
			assert.False(t, cachePolicyContains(e.GetPolicy(), tt.rule),
				"TxEnforcer.AddPolicy must not mutate cache")

			// Persistence via reload: a fresh enforcer must see the rule.
			fresh, err := NewEnforcer(e.policies)
			require.NoError(t, err)
			assert.True(t, cachePolicyContains(fresh.GetPolicy(), tt.rule),
				"rule must be persisted to bbolt")
		})
	}
}

func TestTxEnforcer_RemovePolicy(t *testing.T) {
	t.Parallel()

	rule := []string{"role:custom", "*", "config", "read"}

	tests := []struct {
		name string
	}{
		{name: "removes from persistence and records op"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			e, txm, policies := newInternalTestEnforcer(t)

			require.NoError(t, policies.AddPolicy("p", "p", rule))

			var captured *TxEnforcer

			err := txm.Write(t.Context(), func(tx storage.Tx) error {
				txe := e.WithTx(tx)
				captured = txe

				return txe.RemovePolicy(rule[0], rule[1], rule[2], rule[3])
			})
			require.NoError(t, err)
			require.NotNil(t, captured)

			require.Len(t, captured.ops, 1)
			assert.Equal(t, opRemoveP, captured.ops[0].kind)
			assert.Equal(t, rule, captured.ops[0].args)

			// Reload to confirm persistence.
			fresh, err := NewEnforcer(e.policies)
			require.NoError(t, err)
			assert.False(t, cachePolicyContains(fresh.GetPolicy(), rule))
		})
	}
}

func TestTxEnforcer_AddRoleForUser(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		user string
		role string
		dom  string
	}{
		{name: "records g-rule op", user: "alice", role: "admin", dom: "*"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			e, txm, _ := newInternalTestEnforcer(t)

			var captured *TxEnforcer

			err := txm.Write(t.Context(), func(tx storage.Tx) error {
				txe := e.WithTx(tx)
				captured = txe

				return txe.AddRoleForUser(tt.user, tt.role, tt.dom)
			})
			require.NoError(t, err)

			require.Len(t, captured.ops, 1)
			assert.Equal(t, opAddG, captured.ops[0].kind)
			assert.Equal(t, []string{tt.user, tt.role, tt.dom}, captured.ops[0].args)

			// Cache untouched.
			assert.False(t, cachePolicyContains(e.GetGroupingPolicy(),
				[]string{tt.user, tt.role, tt.dom}))

			// Persisted.
			fresh, err := NewEnforcer(e.policies)
			require.NoError(t, err)
			assert.True(t, cachePolicyContains(fresh.GetGroupingPolicy(),
				[]string{tt.user, tt.role, tt.dom}))
		})
	}
}

func TestTxEnforcer_RemoveRoleForUser(t *testing.T) {
	t.Parallel()

	gRule := []string{"alice", "admin", "*"}

	tests := []struct {
		name string
	}{
		{name: "removes existing g-rule from persistence"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			e, txm, policies := newInternalTestEnforcer(t)

			require.NoError(t, policies.AddPolicy("g", "g", gRule))

			var captured *TxEnforcer

			err := txm.Write(t.Context(), func(tx storage.Tx) error {
				txe := e.WithTx(tx)
				captured = txe

				return txe.RemoveRoleForUser(gRule[0], gRule[1], gRule[2])
			})
			require.NoError(t, err)

			require.Len(t, captured.ops, 1)
			assert.Equal(t, opRemoveG, captured.ops[0].kind)

			fresh, err := NewEnforcer(e.policies)
			require.NoError(t, err)
			assert.False(t, cachePolicyContains(fresh.GetGroupingPolicy(), gRule))
		})
	}
}

func TestTxEnforcer_DeleteUser(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		email    string
		preSeed  [][]string // g-rules to seed
		mustGone [][]string // rules expected to be removed
		mustStay [][]string // unrelated rules that must remain
	}{
		{
			name:  "removes user from subject and role columns",
			email: "alice@example.com",
			preSeed: [][]string{
				{"alice@example.com", "admin", "*"},              // col 0 (subject)
				{"bob@example.com", "alice@example.com", "prod"}, // col 1 (role/group target)
				{"carol@example.com", "writer", "*"},             // unrelated
			},
			mustGone: [][]string{
				{"alice@example.com", "admin", "*"},
				{"bob@example.com", "alice@example.com", "prod"},
			},
			mustStay: [][]string{
				{"carol@example.com", "writer", "*"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			e, txm, policies := newInternalTestEnforcer(t)

			for _, r := range tt.preSeed {
				require.NoError(t, policies.AddPolicy("g", "g", r))
			}

			var captured *TxEnforcer

			err := txm.Write(t.Context(), func(tx storage.Tx) error {
				txe := e.WithTx(tx)
				captured = txe

				return txe.DeleteUser(tt.email)
			})
			require.NoError(t, err)

			require.Len(t, captured.ops, 1)
			assert.Equal(t, opDeleteUser, captured.ops[0].kind)
			assert.Equal(t, tt.email, captured.ops[0].user)

			fresh, err := NewEnforcer(e.policies)
			require.NoError(t, err)

			grouping := fresh.GetGroupingPolicy()

			for _, gone := range tt.mustGone {
				assert.False(t, cachePolicyContains(grouping, gone),
					"rule must be removed: %v", gone)
			}

			for _, stay := range tt.mustStay {
				assert.True(t, cachePolicyContains(grouping, stay),
					"unrelated rule must remain: %v", stay)
			}
		})
	}
}

func TestTxEnforcer_MultipleOpsRecordedInOrder(t *testing.T) {
	t.Parallel()

	e, txm, _ := newInternalTestEnforcer(t)

	var captured *TxEnforcer

	err := txm.Write(t.Context(), func(tx storage.Tx) error {
		txe := e.WithTx(tx)
		captured = txe

		if err := txe.AddPolicy("role:x", "*", "config", "read"); err != nil {
			return err
		}
		if err := txe.AddRoleForUser("alice", "role:x", "*"); err != nil {
			return err
		}

		return txe.RemovePolicy("role:x", "*", "config", "read")
	})
	require.NoError(t, err)

	require.Len(t, captured.ops, 3)
	assert.Equal(t, opAddP, captured.ops[0].kind)
	assert.Equal(t, opAddG, captured.ops[1].kind)
	assert.Equal(t, opRemoveP, captured.ops[2].kind)
}

// sentinel for negative-path cache-sync test in enforcer_internal_test.
var errSentinelTx = errors.New("sentinel tx failure")

package casbin

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/internal/storage/bbolt"
	policyrepo "github.com/sergeyslonimsky/elara/internal/storage/bbolt/policy"
	pkgbbolt "github.com/sergeyslonimsky/elara/pkg/bbolt"
)

// freshEnforcerAndManager creates a new bbolt-backed Enforcer + Manager and
// returns the underlying PolicyRepo for out-of-band manipulations.
func freshEnforcerAndManager(t *testing.T) (*Enforcer, *bbolt.Manager, *policyrepo.Repository) {
	t.Helper()

	path := filepath.Join(t.TempDir(), "casbin.db")

	store, err := bbolt.Open(path)
	require.NoError(t, err)

	t.Cleanup(func() { _ = store.Close() })

	storageManager := bbolt.NewManager(store.DB())
	policies := policyrepo.NewRepository(pkgbbolt.NewManager(store.DB()))

	e, err := NewEnforcer(policies)
	require.NoError(t, err)

	return e, storageManager, policies
}

func TestEnforcer_WriteTx(t *testing.T) {
	t.Parallel()

	pRule := []string{"role:scoped", "ns1", "config", "read"}
	gRule := []string{"alice", "role:scoped", "ns1"}

	tests := []struct {
		name     string
		fn       func(ctx context.Context, txe *TxEnforcer) error
		wantErr  string
		wantP    [][]string // p-rules expected after WriteTx (subset)
		notWantP [][]string // p-rules expected NOT to be present
		wantG    [][]string
		notWantG [][]string
	}{
		{
			name: "success syncs cache",
			fn: func(_ context.Context, txe *TxEnforcer) error {
				if err := txe.AddPolicy(pRule[0], pRule[1], pRule[2], pRule[3]); err != nil {
					return err
				}

				return txe.AddRoleForUser(gRule[0], gRule[1], gRule[2])
			},
			wantP: [][]string{pRule},
			wantG: [][]string{gRule},
		},
		{
			name: "fn error rolls back and does not sync cache",
			fn: func(_ context.Context, txe *TxEnforcer) error {
				_ = txe.AddPolicy(pRule[0], pRule[1], pRule[2], pRule[3])

				return errSentinelTx
			},
			wantErr:  "sentinel tx failure",
			notWantP: [][]string{pRule},
		},
		{
			name: "atomicity: error on second op rolls back both",
			fn: func(_ context.Context, txe *TxEnforcer) error {
				if err := txe.AddPolicy(pRule[0], pRule[1], pRule[2], pRule[3]); err != nil {
					return err
				}
				_ = txe.AddRoleForUser(gRule[0], gRule[1], gRule[2])

				return errSentinelTx
			},
			wantErr:  "sentinel tx failure",
			notWantP: [][]string{pRule},
			notWantG: [][]string{gRule},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			e, txm, policies := freshEnforcerAndManager(t)

			err := e.WriteTx(t.Context(), txm, tt.fn)

			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
			}

			// Cache assertions.
			cacheP := e.GetPolicy()
			cacheG := e.GetGroupingPolicy()
			for _, want := range tt.wantP {
				assert.True(t, cachePolicyContains(cacheP, want),
					"cache must contain p-rule %v", want)
			}
			for _, want := range tt.wantG {
				assert.True(t, cachePolicyContains(cacheG, want),
					"cache must contain g-rule %v", want)
			}
			for _, want := range tt.notWantP {
				assert.False(t, cachePolicyContains(cacheP, want),
					"cache must NOT contain p-rule %v", want)
			}
			for _, want := range tt.notWantG {
				assert.False(t, cachePolicyContains(cacheG, want),
					"cache must NOT contain g-rule %v", want)
			}

			// Persistence assertions via fresh enforcer load.
			fresh, err := NewEnforcer(policies)
			require.NoError(t, err)

			persistedP := fresh.GetPolicy()
			persistedG := fresh.GetGroupingPolicy()
			for _, want := range tt.wantP {
				assert.True(t, cachePolicyContains(persistedP, want),
					"persistence must contain p-rule %v", want)
			}
			for _, want := range tt.notWantP {
				assert.False(t, cachePolicyContains(persistedP, want),
					"persistence must NOT contain p-rule %v", want)
			}
			for _, want := range tt.wantG {
				assert.True(t, cachePolicyContains(persistedG, want),
					"persistence must contain g-rule %v", want)
			}
			for _, want := range tt.notWantG {
				assert.False(t, cachePolicyContains(persistedG, want),
					"persistence must NOT contain g-rule %v", want)
			}
		})
	}
}

// TestEnforcer_WriteTx_DeleteUser_SyncsRolePositionCache is a regression test
// for applyOpsToCache's opDeleteUser branch: gocasbin's DeleteUser only
// strips g-rules where the deleted identifier is the subject (column 0), not
// the role/group target (column 1). TxEnforcer.DeleteUser removes both on
// disk, so the in-memory cache must mirror that or it goes stale relative to
// bbolt — e.g. PAP.GroupMembers would keep returning a deleted group's
// members until the process restarts and reloads policy.
func TestEnforcer_WriteTx_DeleteUser_SyncsRolePositionCache(t *testing.T) {
	t.Parallel()

	e, txm, _ := freshEnforcerAndManager(t)

	// "alice" is a member of group "devs" -- devs sits in the role/target
	// position (column 1) of this g-rule, not the subject position.
	require.NoError(t, e.WriteTx(t.Context(), txm, func(_ context.Context, txe *TxEnforcer) error {
		return txe.AddRoleForUser("alice", "devs", "*")
	}))
	require.True(t, cachePolicyContains(e.GetGroupingPolicy(), []string{"alice", "devs", "*"}))

	require.NoError(t, e.WriteTx(t.Context(), txm, func(_ context.Context, txe *TxEnforcer) error {
		return txe.DeleteUser("devs")
	}))

	assert.False(t, cachePolicyContains(e.GetGroupingPolicy(), []string{"alice", "devs", "*"}),
		"cache must not retain a g-rule naming the deleted identifier in the role position")
}

func TestEnforcer_WriteTx_EnforceSeesRulesAfterCommit(t *testing.T) {
	t.Parallel()

	e, txm, _ := freshEnforcerAndManager(t)

	err := e.WriteTx(t.Context(), txm, func(ctx context.Context, txe *TxEnforcer) error {
		// Attach a wildcard capability bundle to the admin subject, then link
		// alice to it — both committed in the same tx for the end-to-end check.
		if err := txe.AddPolicy("admin", "*", "*", "*"); err != nil {
			return err
		}

		return txe.AddRoleForUser("alice", "admin", "*")
	})
	require.NoError(t, err)

	ok, err := e.Enforce("alice", "*", "config", "write")
	require.NoError(t, err)
	assert.True(t, ok, "alice should be able to write after admin role added via WriteTx")
}

func TestEnforcer_LoadPolicy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		gRule     []string
		wantAllow bool
	}{
		{
			name:      "resyncs cache after out-of-band write",
			gRule:     []string{"alice", "admin", "*"},
			wantAllow: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			e, _, policies := freshEnforcerAndManager(t)

			// Out-of-band: write the admin capability bundle and a g-rule
			// directly via PolicyRepo (bypasses the enforcer cache).
			require.NoError(t, policies.AddPolicyCtx(t.Context(), "p", "p", []string{"admin", "*", "*", "*"}))
			require.NoError(t, policies.AddPolicyCtx(t.Context(), "g", "g", tt.gRule))

			// Before LoadPolicy: cache does not see the rule.
			ok, err := e.Enforce(tt.gRule[0], "*", "config", "write")
			require.NoError(t, err)
			require.False(t, ok, "cache should not see rule before LoadPolicy")

			// Resync.
			require.NoError(t, e.LoadPolicy())

			// After LoadPolicy: cache reflects bbolt state.
			ok, err = e.Enforce(tt.gRule[0], "*", "config", "write")
			require.NoError(t, err)
			assert.Equal(t, tt.wantAllow, ok)
		})
	}
}

// AddRoleForUser / RemoveRoleForUser bridge methods remain on Enforcer
// until the policy usecase migrates to TxEnforcer (EL-4 M5). Their
// persistence/cache parity is covered indirectly by the policy usecase
// tests; no dedicated bridge-level test is kept here.

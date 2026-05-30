package bbolt_test

import (
	"testing"

	"github.com/casbin/casbin/v2/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bboltadapter "github.com/sergeyslonimsky/elara/internal/service/adapter/bbolt"
)

//nolint:lll //casbin rule
const testCasbinModel = `
[request_definition]
r = sub, dom, obj, act

[policy_definition]
p = sub, dom, obj, act

[role_definition]
g = _, _, _

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = g(r.sub, p.sub, r.dom) && (r.dom == p.dom || p.dom == "*") && keyMatch(r.obj, p.obj) && (r.act == p.act || p.act == "*")
`

func newTestModel(t *testing.T) model.Model {
	t.Helper()

	m, err := model.NewModelFromString(testCasbinModel)
	require.NoError(t, err)

	return m
}

func TestPolicyRepo_LoadPolicy_Empty(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	repo := bboltadapter.NewPolicyRepo(store)

	m := newTestModel(t)
	err := repo.LoadPolicy(m)
	require.NoError(t, err)
}

func TestPolicyRepo_SaveAndLoad(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	repo := bboltadapter.NewPolicyRepo(store)

	// Build a model with some rules and save it.
	m := newTestModel(t)
	require.NoError(t, m.AddPolicy("p", "p", []string{"admin", "*", "*", "*"}))
	require.NoError(t, m.AddPolicy("p", "p", []string{"reader", "*", "config", "read"}))
	require.NoError(t, m.AddPolicy("g", "g", []string{"alice", "admin", "*"}))
	require.NoError(t, repo.SavePolicy(m))

	// Load into a fresh model and verify rules were persisted.
	m2 := newTestModel(t)
	require.NoError(t, repo.LoadPolicy(m2))

	pRules := m2["p"]["p"].Policy
	assert.Len(t, pRules, 2)

	gRules := m2["g"]["g"].Policy
	assert.Len(t, gRules, 1)
	assert.Equal(t, []string{"alice", "admin", "*"}, gRules[0])
}

func TestPolicyRepo_SavePolicy_Overwrites(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	repo := bboltadapter.NewPolicyRepo(store)

	m := newTestModel(t)
	require.NoError(t, m.AddPolicy("p", "p", []string{"admin", "*", "*", "*"}))
	require.NoError(t, repo.SavePolicy(m))

	// Save a different model — old rules must be gone.
	m2 := newTestModel(t)
	require.NoError(t, m2.AddPolicy("p", "p", []string{"reader", "*", "config", "read"}))
	require.NoError(t, repo.SavePolicy(m2))

	m3 := newTestModel(t)
	require.NoError(t, repo.LoadPolicy(m3))

	pRules := m3["p"]["p"].Policy
	require.Len(t, pRules, 1)
	assert.Equal(t, []string{"reader", "*", "config", "read"}, pRules[0])
}

func TestPolicyRepo_AddPolicy(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	repo := bboltadapter.NewPolicyRepo(store)

	require.NoError(t, repo.AddPolicy("p", "p", []string{"admin", "*", "*", "*"}))
	require.NoError(t, repo.AddPolicy("g", "g", []string{"alice", "admin", "*"}))

	m := newTestModel(t)
	require.NoError(t, repo.LoadPolicy(m))

	pRules := m["p"]["p"].Policy
	require.Len(t, pRules, 1)
	assert.Equal(t, []string{"admin", "*", "*", "*"}, pRules[0])

	gRules := m["g"]["g"].Policy
	require.Len(t, gRules, 1)
	assert.Equal(t, []string{"alice", "admin", "*"}, gRules[0])
}

func TestPolicyRepo_RemovePolicy(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	repo := bboltadapter.NewPolicyRepo(store)

	require.NoError(t, repo.AddPolicy("p", "p", []string{"admin", "*", "*", "*"}))
	require.NoError(t, repo.AddPolicy("p", "p", []string{"reader", "*", "config", "read"}))

	// Remove one rule.
	require.NoError(t, repo.RemovePolicy("p", "p", []string{"admin", "*", "*", "*"}))

	m := newTestModel(t)
	require.NoError(t, repo.LoadPolicy(m))

	pRules := m["p"]["p"].Policy
	require.Len(t, pRules, 1)
	assert.Equal(t, []string{"reader", "*", "config", "read"}, pRules[0])
}

func TestPolicyRepo_RemoveFilteredPolicy(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	repo := bboltadapter.NewPolicyRepo(store)

	require.NoError(t, repo.AddPolicy("g", "g", []string{"alice", "admin", "*"}))
	require.NoError(t, repo.AddPolicy("g", "g", []string{"bob", "reader", "*"}))
	require.NoError(t, repo.AddPolicy("g", "g", []string{"carol", "admin", "prod"}))

	// Remove all g rules where field[0] == "alice".
	require.NoError(t, repo.RemoveFilteredPolicy("g", "g", 0, "alice"))

	m := newTestModel(t)
	require.NoError(t, repo.LoadPolicy(m))

	gRules := m["g"]["g"].Policy
	assert.Len(t, gRules, 2)

	for _, r := range gRules {
		assert.NotEqual(t, "alice", r[0], "alice rule should have been removed")
	}
}

func TestPolicyRepo_AddPolicy_Idempotent(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	repo := bboltadapter.NewPolicyRepo(store)

	require.NoError(t, repo.AddPolicy("p", "p", []string{"admin", "*", "*", "*"}))
	require.NoError(t, repo.AddPolicy("p", "p", []string{"admin", "*", "*", "*"}))

	m := newTestModel(t)
	require.NoError(t, repo.LoadPolicy(m))

	// bbolt Put is idempotent — same key overwrites itself, still one entry.
	pRules := m["p"]["p"].Policy
	assert.Len(t, pRules, 1)
}

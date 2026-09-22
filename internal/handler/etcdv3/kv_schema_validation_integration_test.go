//go:build integration

// Ш1 verification (plans/etcd-usecase-decoupling/plan.md): a Put through the
// etcd-compatible gRPC API into a namespace with an attached JSON Schema
// must be rejected exactly like the ConnectRPC path — this is finding #1's
// fix, "schema validation does not run on etcd writes".
package etcdv3_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/sergeyslonimsky/elara/internal/domain"
	itest "github.com/sergeyslonimsky/elara/test/integration"
)

func TestIntegration_KVSchemaValidation_RejectsInvalidJSON(t *testing.T) {
	t.Parallel()

	s := itest.New(t)
	cli := s.StartEtcd(t)
	ctx := t.Context()

	require.NoError(t, s.Adapters.SchemaRepo.Attach(ctx, &domain.SchemaAttachment{
		ID:          "schema-test",
		Namespace:   "prod",
		PathPattern: "/*.json",
		JSONSchema:  `{"type":"object","required":["host"],"properties":{"host":{"type":"string"}}}`,
		CreatedAt:   time.Now(),
	}))

	_, err := cli.Put(ctx, "/prod/schema-bad.json", `{"missing":"host"}`)
	require.Error(
		t,
		err,
		"Put must be rejected: content violates the schema, same as ConnectRPC's Config.Create/Update",
	)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))

	get, err := cli.Get(ctx, "/prod/schema-bad.json")
	require.NoError(t, err)
	assert.Empty(t, get.Kvs, "rejected write must not land in storage")
}

func TestIntegration_KVSchemaValidation_AcceptsMatchingJSON(t *testing.T) {
	t.Parallel()

	s := itest.New(t)
	cli := s.StartEtcd(t)
	ctx := t.Context()

	require.NoError(t, s.Adapters.SchemaRepo.Attach(ctx, &domain.SchemaAttachment{
		ID:          "schema-test",
		Namespace:   "prod",
		PathPattern: "/*.json",
		JSONSchema:  `{"type":"object","required":["host"],"properties":{"host":{"type":"string"}}}`,
		CreatedAt:   time.Now(),
	}))

	_, err := cli.Put(ctx, "/prod/schema-ok.json", `{"host":"api.prod.example.com"}`)
	require.NoError(t, err)

	get, err := cli.Get(ctx, "/prod/schema-ok.json")
	require.NoError(t, err)
	require.Len(t, get.Kvs, 1)
	assert.JSONEq(t, `{"host":"api.prod.example.com"}`, string(get.Kvs[0].Value))
}

// TestIntegration_KVSchemaValidation_NoSchemaAttached_Unaffected guards
// against a regression where schema validation accidentally rejects writes
// in namespaces with no attached schema — the common case, and what all
// the Ш0 baseline conformance tests already rely on implicitly.
func TestIntegration_KVSchemaValidation_NoSchemaAttached_Unaffected(t *testing.T) {
	t.Parallel()

	s := itest.New(t)
	cli := s.StartEtcd(t)
	ctx := t.Context()

	_, err := cli.Put(ctx, "/prod/no-schema.json", `{"anything":"goes"}`)
	require.NoError(t, err)
}

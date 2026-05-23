//go:build integration

// Default-persona authz coverage lives in each handler's own *_integration_test.go.
// This file collects the scenarios that exercise the Create-vs-Write distinction
// introduced by T9.0 plus the per-resource scoping invariants from T9.4 and T9.2 —
// they require ad-hoc personas built via Suite.AddPersona, which is why they sit
// here rather than in the handler-specific suites.
package v2_test

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/proto/elara/group/v1/groupv1connect"
	"github.com/sergeyslonimsky/elara/internal/proto/elara/namespace/v1/namespacev1connect"
	itest "github.com/sergeyslonimsky/elara/test/integration"
)

// TestIntegration_M9_DefaultPersonasCannotCreateGroup asserts that none of the
// namespace-scoped personas (devops/developer/tester/no-access) can create
// groups. CreateGroup requires `(Group, Create, *)`, which only superadmin has.
//
// This is the negative-path for the new ActionCreate gate at the global scope.
func TestIntegration_M9_DefaultPersonasCannotCreateGroup(t *testing.T) {
	t.Parallel()

	endpoint := groupv1connect.GroupServiceCreateGroupProcedure
	body := mustJSON(t, map[string]any{"name": "engineering"})

	for _, persona := range []string{"devops", "developer", "tester", "no-access"} {
		t.Run(persona+"_denied", func(t *testing.T) {
			t.Parallel()

			s := itest.New(t)
			resp := itest.DoRequest(t, s, endpoint, body, itest.WithPersona(s, persona))
			defer func() { _ = resp.Body.Close() }()

			requireConnectCode(t, resp, "permission_denied")
		})
	}
}

// TestIntegration_M9_CreatorOnly_CanCreateButNotUpdate verifies the Create-vs-Write
// distinction at the cross-handler level: a persona granted only `(Group, Create, *)`
// can call CreateGroup but receives 403 on UpdateGroup of an existing group.
//
// Without the ActionCreate split (T9.0), this scenario was inexpressible —
// granting Group:Write would have also allowed updating existing groups.
func TestIntegration_M9_CreatorOnly_CanCreateButNotUpdate(t *testing.T) {
	t.Parallel()

	s := itest.New(t)

	creatorToken := s.AddPersona(t, "creator@example.com", "creators", []itest.GroupPerm{
		{Group: "creators", Object: domain.ObjectGroup, Action: domain.ActionCreate, Domain: domain.DomainAll},
	})

	// 1. Create new group succeeds — creator has Group:Create on the global scope.
	createResp := itest.DoRequest(t, s,
		groupv1connect.GroupServiceCreateGroupProcedure,
		mustJSON(t, map[string]any{"name": "engineering"}),
		itest.WithToken(creatorToken),
	)
	createBody := drainBody(t, createResp)
	require.NoError(t, createResp.Body.Close())
	require.Equalf(t, http.StatusOK, createResp.StatusCode,
		"creator should be able to create group; body=%s", createBody)

	createdID := extractGroupIDFromBody(t, createBody)

	// 2. Attempting to UpdateGroup the just-created group fails — creator has no
	// (Group, Write, group:<id>) grant. CreateGroup didn't auto-grant Write.
	updateResp := itest.DoRequest(t, s,
		groupv1connect.GroupServiceUpdateGroupProcedure,
		mustJSON(t, map[string]any{
			"id":      createdID,
			"name":    "engineering-renamed",
			"version": 1,
		}),
		itest.WithToken(creatorToken),
	)
	defer func() { _ = updateResp.Body.Close() }()

	requireConnectCode(t, updateResp, "permission_denied")
}

// TestIntegration_M9_NamespaceCreatorOnly_CannotDelete verifies the same
// Create-vs-Write distinction for namespaces: a persona with
// `(Namespace, Create, *)` + `(Namespace, Read, *)` can create namespaces but
// cannot delete them — Delete requires ActionWrite, which is a separate grant.
func TestIntegration_M9_NamespaceCreatorOnly_CannotDelete(t *testing.T) {
	t.Parallel()

	s := itest.New(t)

	creatorToken := s.AddPersona(t, "ns-creator@example.com", "ns-creators", []itest.GroupPerm{
		{Group: "ns-creators", Object: domain.ObjectNamespace, Action: domain.ActionCreate, Domain: domain.DomainAll},
		{Group: "ns-creators", Object: domain.ObjectNamespace, Action: domain.ActionRead, Domain: domain.DomainAll},
	})

	// CreateNamespace succeeds.
	createResp := itest.DoRequest(t, s,
		namespacev1connect.NamespaceServiceCreateNamespaceProcedure,
		mustJSON(t, map[string]any{"name": "experimental"}),
		itest.WithToken(creatorToken),
	)
	createBody := drainBody(t, createResp)
	require.NoError(t, createResp.Body.Close())
	require.Equalf(t, http.StatusOK, createResp.StatusCode,
		"creator should be able to create namespace; body=%s", createBody)

	// DeleteNamespace requires (Namespace, Write, "experimental") — creator lacks it.
	deleteResp := itest.DoRequest(t, s,
		namespacev1connect.NamespaceServiceDeleteNamespaceProcedure,
		mustJSON(t, map[string]any{"name": "experimental"}),
		itest.WithToken(creatorToken),
	)
	defer func() { _ = deleteResp.Body.Close() }()

	requireConnectCode(t, deleteResp, "permission_denied")
}

// --- helpers --------------------------------------------------------------

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	out, err := json.Marshal(v)
	require.NoError(t, err)
	return out
}

func drainBody(t *testing.T, resp *http.Response) string {
	t.Helper()
	b, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return string(b)
}

func requireConnectCode(t *testing.T, resp *http.Response, code string) {
	t.Helper()
	body := drainBody(t, resp)
	require.Containsf(t, body, code,
		"expected connect code %q in response body, got status=%d body=%s",
		code, resp.StatusCode, body)
}

// extractGroupIDFromBody parses a CreateGroupResponse body string and returns
// the new group's id. Fails the test on parse error or missing field.
func extractGroupIDFromBody(t *testing.T, body string) string {
	t.Helper()
	var parsed struct {
		Group struct {
			ID string `json:"id"`
		} `json:"group"`
	}
	require.NoError(t, json.Unmarshal([]byte(body), &parsed),
		"parsing CreateGroupResponse body: %s", body)
	require.NotEmpty(t, parsed.Group.ID, "group.id missing in body: %s", body)
	require.False(t, strings.Contains(body, "\"code\":"),
		"expected success body, got error envelope: %s", body)
	return parsed.Group.ID
}

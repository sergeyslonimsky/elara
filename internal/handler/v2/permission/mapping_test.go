package permission_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/handler/v2/permission"
	commonv1 "github.com/sergeyslonimsky/elara/internal/proto/elara/common/v1"
)

func TestObjectToProto(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want commonv1.PermissionObject
	}{
		{name: "namespace", in: domain.ObjectNamespace, want: commonv1.PermissionObject_PERMISSION_OBJECT_NAMESPACE},
		{name: "config", in: domain.ObjectConfig, want: commonv1.PermissionObject_PERMISSION_OBJECT_CONFIG},
		{name: "user", in: domain.ObjectUser, want: commonv1.PermissionObject_PERMISSION_OBJECT_USER},
		{name: "group", in: domain.ObjectGroup, want: commonv1.PermissionObject_PERMISSION_OBJECT_GROUP},
		{name: "token", in: domain.ObjectToken, want: commonv1.PermissionObject_PERMISSION_OBJECT_TOKEN},
		{name: "webhook", in: domain.ObjectWebhook, want: commonv1.PermissionObject_PERMISSION_OBJECT_WEBHOOK},
		{name: "all wildcard", in: domain.ObjectAll, want: commonv1.PermissionObject_PERMISSION_OBJECT_ALL},
		{name: "unknown", in: "what-is-this", want: commonv1.PermissionObject_PERMISSION_OBJECT_UNSPECIFIED},
		{name: "empty", in: "", want: commonv1.PermissionObject_PERMISSION_OBJECT_UNSPECIFIED},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, permission.ObjectToProto(tt.in))
		})
	}
}

func TestObjectToDomain(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   commonv1.PermissionObject
		want string
	}{
		{name: "unspecified", in: commonv1.PermissionObject_PERMISSION_OBJECT_UNSPECIFIED, want: ""},
		{name: "namespace", in: commonv1.PermissionObject_PERMISSION_OBJECT_NAMESPACE, want: domain.ObjectNamespace},
		{name: "config", in: commonv1.PermissionObject_PERMISSION_OBJECT_CONFIG, want: domain.ObjectConfig},
		{name: "user", in: commonv1.PermissionObject_PERMISSION_OBJECT_USER, want: domain.ObjectUser},
		{name: "group", in: commonv1.PermissionObject_PERMISSION_OBJECT_GROUP, want: domain.ObjectGroup},
		{name: "token", in: commonv1.PermissionObject_PERMISSION_OBJECT_TOKEN, want: domain.ObjectToken},
		{name: "webhook", in: commonv1.PermissionObject_PERMISSION_OBJECT_WEBHOOK, want: domain.ObjectWebhook},
		{name: "all wildcard", in: commonv1.PermissionObject_PERMISSION_OBJECT_ALL, want: domain.ObjectAll},
		{name: "unknown int", in: commonv1.PermissionObject(999), want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, permission.ObjectToDomain(tt.in))
		})
	}
}

func TestActionToProto(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want commonv1.PermissionAction
	}{
		{name: "read", in: domain.ActionRead, want: commonv1.PermissionAction_PERMISSION_ACTION_READ},
		{name: "write", in: domain.ActionWrite, want: commonv1.PermissionAction_PERMISSION_ACTION_WRITE},
		{name: "all wildcard", in: domain.ActionAll, want: commonv1.PermissionAction_PERMISSION_ACTION_ALL},
		{name: "unknown", in: "delete", want: commonv1.PermissionAction_PERMISSION_ACTION_UNSPECIFIED},
		{name: "empty", in: "", want: commonv1.PermissionAction_PERMISSION_ACTION_UNSPECIFIED},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, permission.ActionToProto(tt.in))
		})
	}
}

func TestActionToDomain(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   commonv1.PermissionAction
		want string
	}{
		{name: "unspecified", in: commonv1.PermissionAction_PERMISSION_ACTION_UNSPECIFIED, want: ""},
		{name: "read", in: commonv1.PermissionAction_PERMISSION_ACTION_READ, want: domain.ActionRead},
		{name: "write", in: commonv1.PermissionAction_PERMISSION_ACTION_WRITE, want: domain.ActionWrite},
		{name: "all wildcard", in: commonv1.PermissionAction_PERMISSION_ACTION_ALL, want: domain.ActionAll},
		{name: "unknown int", in: commonv1.PermissionAction(999), want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, permission.ActionToDomain(tt.in))
		})
	}
}

func TestObjectRoundTrip(t *testing.T) {
	t.Parallel()

	// PERMISSION_OBJECT_UNSPECIFIED round-trips through "" → UNSPECIFIED.
	for _, v := range []commonv1.PermissionObject{
		commonv1.PermissionObject_PERMISSION_OBJECT_UNSPECIFIED,
		commonv1.PermissionObject_PERMISSION_OBJECT_NAMESPACE,
		commonv1.PermissionObject_PERMISSION_OBJECT_CONFIG,
		commonv1.PermissionObject_PERMISSION_OBJECT_USER,
		commonv1.PermissionObject_PERMISSION_OBJECT_GROUP,
		commonv1.PermissionObject_PERMISSION_OBJECT_TOKEN,
		commonv1.PermissionObject_PERMISSION_OBJECT_WEBHOOK,
		commonv1.PermissionObject_PERMISSION_OBJECT_ALL,
	} {
		assert.Equal(t, v, permission.ObjectToProto(permission.ObjectToDomain(v)), "value %s", v)
	}
}

func TestActionRoundTrip(t *testing.T) {
	t.Parallel()

	for _, v := range []commonv1.PermissionAction{
		commonv1.PermissionAction_PERMISSION_ACTION_UNSPECIFIED,
		commonv1.PermissionAction_PERMISSION_ACTION_READ,
		commonv1.PermissionAction_PERMISSION_ACTION_WRITE,
		commonv1.PermissionAction_PERMISSION_ACTION_ALL,
	} {
		assert.Equal(t, v, permission.ActionToProto(permission.ActionToDomain(v)), "value %s", v)
	}
}

func TestAssignmentToProto(t *testing.T) {
	t.Parallel()

	got := permission.AssignmentToProto(domain.Permission{
		Object: domain.ObjectConfig,
		Action: domain.ActionWrite,
		Domain: "ns1",
	})

	require.NotNil(t, got)
	assert.Equal(t, commonv1.PermissionObject_PERMISSION_OBJECT_CONFIG, got.GetObject())
	assert.Equal(t, commonv1.PermissionAction_PERMISSION_ACTION_WRITE, got.GetAction())
	assert.Equal(t, "ns1", got.GetDomain())
}

func TestAssignmentToDomain(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		in     *commonv1.PermissionAssignment
		want   domain.Permission
		wantOK bool
	}{
		{
			name:   "nil input",
			in:     nil,
			wantOK: false,
		},
		{
			name: "unspecified object",
			in: &commonv1.PermissionAssignment{
				Object: commonv1.PermissionObject_PERMISSION_OBJECT_UNSPECIFIED,
				Action: commonv1.PermissionAction_PERMISSION_ACTION_READ,
				Domain: "ns1",
			},
			wantOK: false,
		},
		{
			name: "unspecified action",
			in: &commonv1.PermissionAssignment{
				Object: commonv1.PermissionObject_PERMISSION_OBJECT_CONFIG,
				Action: commonv1.PermissionAction_PERMISSION_ACTION_UNSPECIFIED,
				Domain: "ns1",
			},
			wantOK: false,
		},
		{
			name: "concrete permission",
			in: &commonv1.PermissionAssignment{
				Object: commonv1.PermissionObject_PERMISSION_OBJECT_CONFIG,
				Action: commonv1.PermissionAction_PERMISSION_ACTION_READ,
				Domain: "ns1",
			},
			want:   domain.Permission{Object: domain.ObjectConfig, Action: domain.ActionRead, Domain: "ns1"},
			wantOK: true,
		},
		{
			name: "all wildcard preserved",
			in: &commonv1.PermissionAssignment{
				Object: commonv1.PermissionObject_PERMISSION_OBJECT_ALL,
				Action: commonv1.PermissionAction_PERMISSION_ACTION_ALL,
				Domain: domain.DomainAll,
			},
			want:   domain.Permission{Object: domain.ObjectAll, Action: domain.ActionAll, Domain: domain.DomainAll},
			wantOK: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, ok := permission.AssignmentToDomain(tt.in)
			assert.Equal(t, tt.wantOK, ok)

			if tt.wantOK {
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestAssignmentsToDomain(t *testing.T) {
	t.Parallel()

	t.Run("nil input returns nil", func(t *testing.T) {
		t.Parallel()
		assert.Nil(t, permission.AssignmentsToDomain(nil))
	})

	t.Run("empty input returns nil", func(t *testing.T) {
		t.Parallel()
		assert.Nil(t, permission.AssignmentsToDomain([]*commonv1.PermissionAssignment{}))
	})

	t.Run("drops invalid entries", func(t *testing.T) {
		t.Parallel()

		in := []*commonv1.PermissionAssignment{
			{
				Object: commonv1.PermissionObject_PERMISSION_OBJECT_UNSPECIFIED,
				Action: commonv1.PermissionAction_PERMISSION_ACTION_READ,
				Domain: "ns1",
			},
			{
				Object: commonv1.PermissionObject_PERMISSION_OBJECT_CONFIG,
				Action: commonv1.PermissionAction_PERMISSION_ACTION_READ,
				Domain: "ns2",
			},
			{
				Object: commonv1.PermissionObject_PERMISSION_OBJECT_ALL,
				Action: commonv1.PermissionAction_PERMISSION_ACTION_ALL,
				Domain: domain.DomainAll,
			},
		}

		assert.Equal(t, []domain.Permission{
			{Object: domain.ObjectConfig, Action: domain.ActionRead, Domain: "ns2"},
			{Object: domain.ObjectAll, Action: domain.ActionAll, Domain: domain.DomainAll},
		}, permission.AssignmentsToDomain(in))
	})
}

func TestAssignmentsToProto(t *testing.T) {
	t.Parallel()

	t.Run("nil input returns nil", func(t *testing.T) {
		t.Parallel()
		assert.Nil(t, permission.AssignmentsToProto(nil))
	})

	t.Run("empty input returns nil", func(t *testing.T) {
		t.Parallel()
		assert.Nil(t, permission.AssignmentsToProto([]domain.Permission{}))
	})

	t.Run("maps all entries including wildcards", func(t *testing.T) {
		t.Parallel()

		in := []domain.Permission{
			{Object: domain.ObjectConfig, Action: domain.ActionRead, Domain: "ns1"},
			{Object: domain.ObjectAll, Action: domain.ActionAll, Domain: domain.DomainAll},
		}

		got := permission.AssignmentsToProto(in)
		require.Len(t, got, 2)
		assert.Equal(t, commonv1.PermissionObject_PERMISSION_OBJECT_CONFIG, got[0].GetObject())
		assert.Equal(t, commonv1.PermissionAction_PERMISSION_ACTION_READ, got[0].GetAction())
		assert.Equal(t, "ns1", got[0].GetDomain())
		assert.Equal(t, commonv1.PermissionObject_PERMISSION_OBJECT_ALL, got[1].GetObject())
		assert.Equal(t, commonv1.PermissionAction_PERMISSION_ACTION_ALL, got[1].GetAction())
		assert.Equal(t, domain.DomainAll, got[1].GetDomain())
	})
}

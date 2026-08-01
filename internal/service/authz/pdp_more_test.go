package authz_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/authz"
	authz_mock "github.com/sergeyslonimsky/elara/internal/service/authz/mocks"
)

func TestPDP_HasGroup(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		mockFunc func(*gomock.Controller) *authz.PDP
		want     bool
	}{
		{
			name: "allowed",
			mockFunc: func(ctrl *gomock.Controller) *authz.PDP {
				m := authz_mock.NewMockenforcer(ctrl)
				m.EXPECT().
					Enforce("alice", domain.GroupResource("devs"), string(domain.ObjectGroup), string(domain.ActionRead)).
					Return(true, nil)

				return authz.NewPDP(m)
			},
			want: true,
		},
		{
			name: "denied",
			mockFunc: func(ctrl *gomock.Controller) *authz.PDP {
				m := authz_mock.NewMockenforcer(ctrl)
				m.EXPECT().
					Enforce("alice", domain.GroupResource("devs"), string(domain.ObjectGroup), string(domain.ActionRead)).
					Return(false, nil)

				return authz.NewPDP(m)
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			pdp := tt.mockFunc(ctrl)

			got := pdp.HasGroup("alice", "devs", domain.ActionRead)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestPDP_HasNamespace(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		mockFunc func(*gomock.Controller) *authz.PDP
		want     bool
	}{
		{
			name: "allowed",
			mockFunc: func(ctrl *gomock.Controller) *authz.PDP {
				m := authz_mock.NewMockenforcer(ctrl)
				m.EXPECT().
					Enforce("alice", domain.NamespaceResource("prod"), string(domain.ObjectNamespace), string(domain.ActionWrite)).
					Return(true, nil)

				return authz.NewPDP(m)
			},
			want: true,
		},
		{
			name: "denied",
			mockFunc: func(ctrl *gomock.Controller) *authz.PDP {
				m := authz_mock.NewMockenforcer(ctrl)
				m.EXPECT().
					Enforce("alice", domain.NamespaceResource("prod"), string(domain.ObjectNamespace), string(domain.ActionWrite)).
					Return(false, nil)

				return authz.NewPDP(m)
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			pdp := tt.mockFunc(ctrl)

			got := pdp.HasNamespace("alice", "prod", domain.ActionWrite)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestPDP_EffectiveNamespaces(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		mockFunc func(*gomock.Controller) *authz.PDP
		want     authz.DomainSet
	}{
		{
			name: "wildcard domain strips to bare wildcard",
			mockFunc: func(ctrl *gomock.Controller) *authz.PDP {
				m := authz_mock.NewMockenforcer(ctrl)
				m.EXPECT().GetImplicitPermissionsForUser("alice").Return([][]string{
					{"alice", "*", string(domain.ObjectNamespace), string(domain.ActionRead)},
				}, nil)

				return authz.NewPDP(m)
			},
			want: authz.DomainSet{Wildcard: true, Explicit: map[string]struct{}{}},
		},
		{
			name: "explicit domains have namespace prefix stripped",
			mockFunc: func(ctrl *gomock.Controller) *authz.PDP {
				m := authz_mock.NewMockenforcer(ctrl)
				m.EXPECT().GetImplicitPermissionsForUser("alice").Return([][]string{
					{
						"alice",
						domain.NamespaceResource("prod"),
						string(domain.ObjectNamespace),
						string(domain.ActionRead),
					},
				}, nil)

				return authz.NewPDP(m)
			},
			want: authz.NewDomainSet("prod"),
		},
		{
			name: "skip permissions returns wildcard",
			mockFunc: func(ctrl *gomock.Controller) *authz.PDP {
				return authz.NewPDP(
					authz_mock.NewMockenforcer(ctrl),
					authz.WithSkipPermissions(true),
				)
			},
			want: authz.DomainSet{Wildcard: true, Explicit: map[string]struct{}{}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			pdp := tt.mockFunc(ctrl)

			got := pdp.EffectiveNamespaces("alice", domain.ActionRead)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestPDP_HasGlobal(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	m := authz_mock.NewMockenforcer(ctrl)
	m.EXPECT().
		Enforce("alice", domain.DomainAll, string(domain.ObjectUser), string(domain.ActionRead)).
		Return(true, nil)

	pdp := authz.NewPDP(m)

	assert.True(t, pdp.HasGlobal("alice", domain.ObjectUser, domain.ActionRead))
}

func TestPDP_HasForGroup(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	m := authz_mock.NewMockenforcer(ctrl)
	m.EXPECT().
		Enforce(domain.GroupResource("devs"), "prod", string(domain.ObjectNamespace), string(domain.ActionRead)).
		Return(true, nil)

	pdp := authz.NewPDP(m)

	got := pdp.HasForGroup(
		"devs",
		domain.Permission{
			Object: domain.ObjectNamespace,
			Action: domain.ActionRead,
			Domain: "prod",
		},
	)
	assert.True(t, got)
}

func TestPDP_Has_SkipPermissions(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	pdp := authz.NewPDP(authz_mock.NewMockenforcer(ctrl), authz.WithSkipPermissions(true))

	got := pdp.Has(
		"alice",
		domain.Permission{Object: domain.ObjectUser, Action: domain.ActionRead, Domain: "*"},
	)
	assert.True(t, got)
}

func TestPDP_ListPermissions_SkipPermissions(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	pdp := authz.NewPDP(authz_mock.NewMockenforcer(ctrl), authz.WithSkipPermissions(true))

	got, err := pdp.ListPermissions("alice")
	require.NoError(t, err)
	assert.Equal(t, []domain.Permission{
		{Object: domain.ObjectAll, Action: domain.ActionAll, Domain: domain.DomainAll},
	}, got)
}

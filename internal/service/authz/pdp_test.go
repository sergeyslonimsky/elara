package authz_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/authz"
	authz_mock "github.com/sergeyslonimsky/elara/internal/service/authz/mocks"
)

func TestPDP_EffectiveDomains(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		principal string
		object    domain.Object
		action    domain.Action
		mockFunc  func(*gomock.Controller) *authz.PDP
		want      authz.DomainSet
	}{
		{
			name:      "multiple permissions",
			principal: "user@example.com",
			object:    "config",
			action:    "read",
			mockFunc: func(ctrl *gomock.Controller) *authz.PDP {
				m := authz_mock.NewMockenforcer(ctrl)
				m.EXPECT().GetImplicitPermissionsForUser("user@example.com").Return([][]string{
					{"user@example.com", "dom1", "config", "read"},
					{"user@example.com", "dom2", "config", "read"},
					{"user@example.com", "dom3", "secret", "read"},
				}, nil)

				return authz.NewPDP(m)
			},
			want: authz.NewDomainSet("dom1", "dom2"),
		},
		{
			name:      "wildcard object",
			principal: "user@example.com",
			object:    "config",
			action:    "read",
			mockFunc: func(ctrl *gomock.Controller) *authz.PDP {
				m := authz_mock.NewMockenforcer(ctrl)
				m.EXPECT().GetImplicitPermissionsForUser("user@example.com").Return([][]string{
					{"user@example.com", "dom1", "*", "read"},
				}, nil)

				return authz.NewPDP(m)
			},
			want: authz.NewDomainSet("dom1"),
		},
		{
			name:      "wildcard action",
			principal: "user@example.com",
			object:    "config",
			action:    "read",
			mockFunc: func(ctrl *gomock.Controller) *authz.PDP {
				m := authz_mock.NewMockenforcer(ctrl)
				m.EXPECT().GetImplicitPermissionsForUser("user@example.com").Return([][]string{
					{"user@example.com", "dom1", "config", "*"},
				}, nil)

				return authz.NewPDP(m)
			},
			want: authz.NewDomainSet("dom1"),
		},
		{
			name:      "wildcard domain",
			principal: "user@example.com",
			object:    "config",
			action:    "read",
			mockFunc: func(ctrl *gomock.Controller) *authz.PDP {
				m := authz_mock.NewMockenforcer(ctrl)
				m.EXPECT().GetImplicitPermissionsForUser("user@example.com").Return([][]string{
					{"user@example.com", "*", "config", "read"},
				}, nil)

				return authz.NewPDP(m)
			},
			want: authz.NewDomainSet("*"),
		},
		{
			name:      "error from enforcer",
			principal: "user@example.com",
			object:    "config",
			action:    "read",
			mockFunc: func(ctrl *gomock.Controller) *authz.PDP {
				m := authz_mock.NewMockenforcer(ctrl)
				m.EXPECT().GetImplicitPermissionsForUser("user@example.com").Return(nil, errors.New("db error"))

				return authz.NewPDP(m)
			},
			want: authz.NewDomainSet(),
		},
		{
			name:      "invalid rule length",
			principal: "user@example.com",
			object:    "config",
			action:    "read",
			mockFunc: func(ctrl *gomock.Controller) *authz.PDP {
				m := authz_mock.NewMockenforcer(ctrl)
				m.EXPECT().GetImplicitPermissionsForUser("user@example.com").Return([][]string{
					{"user@example.com", "dom1", "config"}, // too short
				}, nil)

				return authz.NewPDP(m)
			},
			want: authz.NewDomainSet(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			pdp := tt.mockFunc(ctrl)

			got := pdp.EffectiveDomains(tt.principal, tt.object, tt.action)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestPDP_ListPermissions(t *testing.T) {
	t.Parallel()

	const principal = "user@example.com"

	tests := []struct {
		name     string
		mockFunc func(*gomock.Controller) *authz.PDP
		wantErr  string
		want     []domain.Permission
	}{
		{
			name: "empty rules returns empty non-nil slice",
			mockFunc: func(ctrl *gomock.Controller) *authz.PDP {
				m := authz_mock.NewMockenforcer(ctrl)
				m.EXPECT().GetImplicitPermissionsForUser(principal).Return([][]string{}, nil)

				return authz.NewPDP(m)
			},
			want: []domain.Permission{},
		},
		{
			name: "single rule mapped to Permission",
			mockFunc: func(ctrl *gomock.Controller) *authz.PDP {
				m := authz_mock.NewMockenforcer(ctrl)
				m.EXPECT().GetImplicitPermissionsForUser(principal).Return([][]string{
					{"sub", "ns1", string(domain.ObjectNamespace), string(domain.ActionRead)},
				}, nil)

				return authz.NewPDP(m)
			},
			want: []domain.Permission{
				{Object: domain.ObjectNamespace, Action: domain.ActionRead, Domain: "ns1"},
			},
		},
		{
			name: "duplicate rules are deduplicated",
			mockFunc: func(ctrl *gomock.Controller) *authz.PDP {
				m := authz_mock.NewMockenforcer(ctrl)
				m.EXPECT().GetImplicitPermissionsForUser(principal).Return([][]string{
					{"sub", "ns1", string(domain.ObjectNamespace), string(domain.ActionRead)},
					{"sub", "ns1", string(domain.ObjectNamespace), string(domain.ActionRead)},
				}, nil)

				return authz.NewPDP(m)
			},
			want: []domain.Permission{
				{Object: domain.ObjectNamespace, Action: domain.ActionRead, Domain: "ns1"},
			},
		},
		{
			name: "sorted by object then action then domain",
			mockFunc: func(ctrl *gomock.Controller) *authz.PDP {
				m := authz_mock.NewMockenforcer(ctrl)
				m.EXPECT().GetImplicitPermissionsForUser(principal).Return([][]string{
					{"sub", "ns2", string(domain.ObjectUser), string(domain.ActionRead)},
					{"sub", "ns1", string(domain.ObjectNamespace), string(domain.ActionWrite)},
					{"sub", "ns1", string(domain.ObjectNamespace), string(domain.ActionRead)},
					{"sub", "ns2", string(domain.ObjectNamespace), string(domain.ActionRead)},
				}, nil)

				return authz.NewPDP(m)
			},
			want: []domain.Permission{
				{Object: domain.ObjectNamespace, Action: domain.ActionRead, Domain: "ns1"},
				{Object: domain.ObjectNamespace, Action: domain.ActionRead, Domain: "ns2"},
				{Object: domain.ObjectNamespace, Action: domain.ActionWrite, Domain: "ns1"},
				{Object: domain.ObjectUser, Action: domain.ActionRead, Domain: "ns2"},
			},
		},
		{
			name: "malformed rule (len<4) is skipped",
			mockFunc: func(ctrl *gomock.Controller) *authz.PDP {
				m := authz_mock.NewMockenforcer(ctrl)
				m.EXPECT().GetImplicitPermissionsForUser(principal).Return([][]string{
					{"sub", "ns1"},
					{"sub", "ns1", string(domain.ObjectNamespace), string(domain.ActionRead)},
				}, nil)

				return authz.NewPDP(m)
			},
			want: []domain.Permission{
				{Object: domain.ObjectNamespace, Action: domain.ActionRead, Domain: "ns1"},
			},
		},
		{
			name: "wildcard tuple preserved as ObjectAll/ActionAll/DomainAll",
			mockFunc: func(ctrl *gomock.Controller) *authz.PDP {
				m := authz_mock.NewMockenforcer(ctrl)
				m.EXPECT().GetImplicitPermissionsForUser(principal).Return([][]string{
					{"sub", domain.DomainAll, string(domain.ObjectAll), string(domain.ActionAll)},
				}, nil)

				return authz.NewPDP(m)
			},
			want: []domain.Permission{
				{Object: domain.ObjectAll, Action: domain.ActionAll, Domain: domain.DomainAll},
			},
		},
		{
			name: "enforcer error is wrapped",
			mockFunc: func(ctrl *gomock.Controller) *authz.PDP {
				m := authz_mock.NewMockenforcer(ctrl)
				m.EXPECT().GetImplicitPermissionsForUser(principal).Return(nil, errors.New("db error"))

				return authz.NewPDP(m)
			},
			wantErr: "list permissions: db error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			pdp := tt.mockFunc(ctrl)

			got, err := pdp.ListPermissions(principal)

			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)

				return
			}
			require.NoError(t, err)
			assert.NotNil(t, got)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestPDP_Has(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		principal string
		perm      domain.Permission
		mockFunc  func(*gomock.Controller) *authz.PDP
		want      bool
	}{
		{
			name:      "authorized",
			principal: "user@example.com",
			perm:      domain.Permission{Object: "config", Action: "read", Domain: "dom1"},
			mockFunc: func(ctrl *gomock.Controller) *authz.PDP {
				m := authz_mock.NewMockenforcer(ctrl)
				m.EXPECT().Enforce("user@example.com", "dom1", "config", "read").Return(true, nil)

				return authz.NewPDP(m)
			},
			want: true,
		},
		{
			name:      "unauthorized",
			principal: "user@example.com",
			perm:      domain.Permission{Object: "config", Action: "read", Domain: "dom1"},
			mockFunc: func(ctrl *gomock.Controller) *authz.PDP {
				m := authz_mock.NewMockenforcer(ctrl)
				m.EXPECT().Enforce("user@example.com", "dom1", "config", "read").Return(false, nil)

				return authz.NewPDP(m)
			},
			want: false,
		},
		{
			name:      "enforcer error",
			principal: "user@example.com",
			perm:      domain.Permission{Object: "config", Action: "read", Domain: "dom1"},
			mockFunc: func(ctrl *gomock.Controller) *authz.PDP {
				m := authz_mock.NewMockenforcer(ctrl)
				m.EXPECT().Enforce("user@example.com", "dom1", "config", "read").Return(false, errors.New("error"))

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

			got := pdp.Has(tt.principal, tt.perm)
			assert.Equal(t, tt.want, got)
		})
	}
}

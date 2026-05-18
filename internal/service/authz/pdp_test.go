package authz_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
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
		object    string
		action    string
		mockFunc  func(*gomock.Controller) *authz.PDP
		want      domain.DomainSet
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
			want: domain.NewDomainSet("dom1", "dom2"),
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
			want: domain.NewDomainSet("dom1"),
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
			want: domain.NewDomainSet("dom1"),
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
			want: domain.NewDomainSet("*"),
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
			want: domain.NewDomainSet(),
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
			want: domain.NewDomainSet(),
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

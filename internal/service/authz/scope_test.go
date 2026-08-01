package authz_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/authz"
	authz_mock "github.com/sergeyslonimsky/elara/internal/service/authz/mocks"
)

// seedMembership puts actor into the named groups via a real PAP write —
// Scope reads group membership through PAP.UserGroupNames, which wraps a
// concrete *casbin.Enforcer and cannot be gomocked (see authz_helpers_test.go).
func seedMembership(t *testing.T, pap *authz.PAP, userID string, groups ...string) {
	t.Helper()

	require.NoError(t, pap.Write(t.Context(), func(_ context.Context, w *authz.PAPTx) error {
		return w.ApplyUserMembershipDeltas(userID, groups, nil)
	}))
}

func TestScope_CanReadUser(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		mockFunc func(*gomock.Controller) *authz.Scope
		want     bool
	}{
		{
			name: "global User:Read grants access without checking groups",
			mockFunc: func(ctrl *gomock.Controller) *authz.Scope {
				enf := authz_mock.NewMockenforcer(ctrl)
				enf.EXPECT().
					Enforce("alice", domain.DomainAll, string(domain.ObjectUser), string(domain.ActionRead)).
					Return(true, nil)

				return authz.NewScope(authz.NewPDP(enf), nil, nil)
			},
			want: true,
		},
		{
			name: "actor shares a readable group with target",
			mockFunc: func(ctrl *gomock.Controller) *authz.Scope {
				pap, _, _ := newTestPAP(t)
				seedMembership(t, pap, "bob", "devs")

				enf := authz_mock.NewMockenforcer(ctrl)
				enf.EXPECT().
					Enforce("alice", domain.DomainAll, string(domain.ObjectUser), string(domain.ActionRead)).
					Return(false, nil)
				enf.EXPECT().
					Enforce("alice", domain.GroupResource("devs"), string(domain.ObjectGroup), string(domain.ActionRead)).
					Return(true, nil)

				groups := authz_mock.NewMockGroupResolver(ctrl)
				groups.EXPECT().Get(t.Context(), "devs").Return(&domain.Group{Name: "devs"}, nil)

				return authz.NewScope(authz.NewPDP(enf), pap, groups)
			},
			want: true,
		},
		{
			name: "actor has no shared readable group",
			mockFunc: func(ctrl *gomock.Controller) *authz.Scope {
				pap, _, _ := newTestPAP(t)
				seedMembership(t, pap, "bob", "devs")

				enf := authz_mock.NewMockenforcer(ctrl)
				enf.EXPECT().
					Enforce("alice", domain.DomainAll, string(domain.ObjectUser), string(domain.ActionRead)).
					Return(false, nil)
				enf.EXPECT().
					Enforce("alice", domain.GroupResource("devs"), string(domain.ObjectGroup), string(domain.ActionRead)).
					Return(false, nil)

				groups := authz_mock.NewMockGroupResolver(ctrl)
				groups.EXPECT().Get(t.Context(), "devs").Return(&domain.Group{Name: "devs"}, nil)

				return authz.NewScope(authz.NewPDP(enf), pap, groups)
			},
			want: false,
		},
		{
			name: "target has no group memberships",
			mockFunc: func(ctrl *gomock.Controller) *authz.Scope {
				pap, _, _ := newTestPAP(t)

				enf := authz_mock.NewMockenforcer(ctrl)
				enf.EXPECT().
					Enforce("alice", domain.DomainAll, string(domain.ObjectUser), string(domain.ActionRead)).
					Return(false, nil)

				return authz.NewScope(authz.NewPDP(enf), pap, authz_mock.NewMockGroupResolver(ctrl))
			},
			want: false,
		},
		{
			name: "group resolution error fails closed",
			mockFunc: func(ctrl *gomock.Controller) *authz.Scope {
				pap, _, _ := newTestPAP(t)
				seedMembership(t, pap, "bob", "ghost")

				enf := authz_mock.NewMockenforcer(ctrl)
				enf.EXPECT().
					Enforce("alice", domain.DomainAll, string(domain.ObjectUser), string(domain.ActionRead)).
					Return(false, nil)

				groups := authz_mock.NewMockGroupResolver(ctrl)
				groups.EXPECT().Get(t.Context(), "ghost").Return(nil, errors.New("not found"))

				return authz.NewScope(authz.NewPDP(enf), pap, groups)
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			sut := tt.mockFunc(ctrl)

			got := sut.CanReadUser(t.Context(), "alice", "bob")
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestScope_FilterVisibleUsers(t *testing.T) {
	t.Parallel()

	t.Run("empty candidates short-circuits", func(t *testing.T) {
		t.Parallel()

		ctrl := gomock.NewController(t)
		enf := authz_mock.NewMockenforcer(ctrl)
		sut := authz.NewScope(authz.NewPDP(enf), nil, nil)

		got := sut.FilterVisibleUsers(t.Context(), "alice", []string{})
		assert.Empty(t, got)
	})

	t.Run("global read returns candidates unfiltered", func(t *testing.T) {
		t.Parallel()

		ctrl := gomock.NewController(t)
		enf := authz_mock.NewMockenforcer(ctrl)
		enf.EXPECT().
			Enforce("alice", domain.DomainAll, string(domain.ObjectUser), string(domain.ActionRead)).
			Return(true, nil)

		sut := authz.NewScope(authz.NewPDP(enf), nil, nil)

		got := sut.FilterVisibleUsers(t.Context(), "alice", []string{"bob", "carol"})
		assert.Equal(t, []string{"bob", "carol"}, got)
	})

	t.Run("filters candidates by group scope", func(t *testing.T) {
		t.Parallel()

		ctrl := gomock.NewController(t)

		pap, _, _ := newTestPAP(t)
		seedMembership(t, pap, "bob", "devs")

		enf := authz_mock.NewMockenforcer(ctrl)
		enf.EXPECT().
			Enforce("alice", domain.DomainAll, string(domain.ObjectUser), string(domain.ActionRead)).
			Return(false, nil)
		enf.EXPECT().
			Enforce("alice", domain.GroupResource("devs"), string(domain.ObjectGroup), string(domain.ActionRead)).
			Return(true, nil)

		groups := authz_mock.NewMockGroupResolver(ctrl)
		groups.EXPECT().Get(t.Context(), "devs").Return(&domain.Group{Name: "devs"}, nil)

		sut := authz.NewScope(authz.NewPDP(enf), pap, groups)

		got := sut.FilterVisibleUsers(t.Context(), "alice", []string{"bob", "carol"})
		assert.Equal(t, []string{"bob"}, got)
	})
}

func TestScope_VisibleUserGroupNames(t *testing.T) {
	t.Parallel()

	t.Run("filters groups by actor's read scope", func(t *testing.T) {
		t.Parallel()

		ctrl := gomock.NewController(t)

		pap, _, _ := newTestPAP(t)
		seedMembership(t, pap, "bob", "devs", "ops")

		enf := authz_mock.NewMockenforcer(ctrl)
		enf.EXPECT().
			Enforce("alice", domain.GroupResource("devs"), string(domain.ObjectGroup), string(domain.ActionRead)).
			Return(true, nil)
		enf.EXPECT().
			Enforce("alice", domain.GroupResource("ops"), string(domain.ObjectGroup), string(domain.ActionRead)).
			Return(false, nil)

		groups := authz_mock.NewMockGroupResolver(ctrl)
		groups.EXPECT().Get(t.Context(), "devs").Return(&domain.Group{Name: "devs"}, nil)
		groups.EXPECT().Get(t.Context(), "ops").Return(&domain.Group{Name: "ops"}, nil)

		sut := authz.NewScope(authz.NewPDP(enf), pap, groups)

		got, err := sut.VisibleUserGroupNames(t.Context(), "alice", "bob")
		require.NoError(t, err)
		assert.Equal(t, []string{"devs"}, got)
	})

	t.Run("resolution error propagates wrapped", func(t *testing.T) {
		t.Parallel()

		ctrl := gomock.NewController(t)

		pap, _, _ := newTestPAP(t)
		seedMembership(t, pap, "bob", "ghost")

		groups := authz_mock.NewMockGroupResolver(ctrl)
		groups.EXPECT().Get(t.Context(), "ghost").Return(nil, errors.New("not found"))

		sut := authz.NewScope(authz.NewPDP(authz_mock.NewMockenforcer(ctrl)), pap, groups)

		_, err := sut.VisibleUserGroupNames(t.Context(), "alice", "bob")
		require.ErrorContains(t, err, "find group by name ghost: not found")
	})
}

func TestScope_RequireMembershipGrant(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		mockFunc func(*gomock.Controller) *authz.Scope
		errIs    error
	}{
		{
			name: "actor holds every permission the group grants",
			mockFunc: func(ctrl *gomock.Controller) *authz.Scope {
				pap, _, _ := newTestPAP(t)
				require.NoError(
					t,
					pap.Write(t.Context(), func(_ context.Context, w *authz.PAPTx) error {
						return w.ApplyPermissionDeltas(
							"devs",
							[]domain.Permission{
								{
									Object: domain.ObjectNamespace,
									Action: domain.ActionRead,
									Domain: "prod",
								},
							},
							nil,
						)
					}),
				)

				enf := authz_mock.NewMockenforcer(ctrl)
				enf.EXPECT().
					Enforce("alice", "prod", string(domain.ObjectNamespace), string(domain.ActionRead)).
					Return(true, nil)

				return authz.NewScope(authz.NewPDP(enf), pap, nil)
			},
		},
		{
			name: "actor missing a permission the group grants is denied",
			mockFunc: func(ctrl *gomock.Controller) *authz.Scope {
				pap, _, _ := newTestPAP(t)
				require.NoError(
					t,
					pap.Write(t.Context(), func(_ context.Context, w *authz.PAPTx) error {
						return w.ApplyPermissionDeltas(
							"devs",
							[]domain.Permission{
								{
									Object: domain.ObjectNamespace,
									Action: domain.ActionWrite,
									Domain: "prod",
								},
							},
							nil,
						)
					}),
				)

				enf := authz_mock.NewMockenforcer(ctrl)
				enf.EXPECT().
					Enforce("alice", "prod", string(domain.ObjectNamespace), string(domain.ActionWrite)).
					Return(false, nil)

				return authz.NewScope(authz.NewPDP(enf), pap, nil)
			},
			errIs: domain.ErrPermissionEscalation,
		},
		{
			name: "group with no permissions never escalates",
			mockFunc: func(ctrl *gomock.Controller) *authz.Scope {
				pap, _, _ := newTestPAP(t)

				return authz.NewScope(authz.NewPDP(authz_mock.NewMockenforcer(ctrl)), pap, nil)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			sut := tt.mockFunc(ctrl)

			err := sut.RequireMembershipGrant("alice", "devs")

			if tt.errIs != nil {
				require.ErrorIs(t, err, tt.errIs)

				return
			}
			require.NoError(t, err)
		})
	}
}

func TestScope_RequireWriteUser(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		mockFunc func(*gomock.Controller) *authz.Scope
		errIs    error
	}{
		{
			name: "global User:Write grants access",
			mockFunc: func(ctrl *gomock.Controller) *authz.Scope {
				enf := authz_mock.NewMockenforcer(ctrl)
				enf.EXPECT().
					Enforce("alice", domain.DomainAll, string(domain.ObjectUser), string(domain.ActionWrite)).
					Return(true, nil)

				return authz.NewScope(authz.NewPDP(enf), nil, nil)
			},
		},
		{
			name: "group write access grants target scope",
			mockFunc: func(ctrl *gomock.Controller) *authz.Scope {
				pap, _, _ := newTestPAP(t)
				seedMembership(t, pap, "bob", "devs")

				enf := authz_mock.NewMockenforcer(ctrl)
				enf.EXPECT().
					Enforce("alice", domain.DomainAll, string(domain.ObjectUser), string(domain.ActionWrite)).
					Return(false, nil)
				enf.EXPECT().
					Enforce("alice", domain.GroupResource("devs"), string(domain.ObjectGroup), string(domain.ActionWrite)).
					Return(true, nil)

				groups := authz_mock.NewMockGroupResolver(ctrl)
				groups.EXPECT().Get(t.Context(), "devs").Return(&domain.Group{Name: "devs"}, nil)

				return authz.NewScope(authz.NewPDP(enf), pap, groups)
			},
		},
		{
			name: "no scope match is forbidden",
			mockFunc: func(ctrl *gomock.Controller) *authz.Scope {
				pap, _, _ := newTestPAP(t)

				enf := authz_mock.NewMockenforcer(ctrl)
				enf.EXPECT().
					Enforce("alice", domain.DomainAll, string(domain.ObjectUser), string(domain.ActionWrite)).
					Return(false, nil)

				return authz.NewScope(authz.NewPDP(enf), pap, authz_mock.NewMockGroupResolver(ctrl))
			},
			errIs: domain.ErrForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			sut := tt.mockFunc(ctrl)

			err := sut.RequireWriteUser(t.Context(), "alice", "bob")

			if tt.errIs != nil {
				require.ErrorIs(t, err, tt.errIs)

				return
			}
			require.NoError(t, err)
		})
	}

	t.Run("resolution error propagates wrapped", func(t *testing.T) {
		t.Parallel()

		ctrl := gomock.NewController(t)

		pap, _, _ := newTestPAP(t)
		seedMembership(t, pap, "bob", "ghost")

		enf := authz_mock.NewMockenforcer(ctrl)
		enf.EXPECT().
			Enforce("alice", domain.DomainAll, string(domain.ObjectUser), string(domain.ActionWrite)).
			Return(false, nil)

		groups := authz_mock.NewMockGroupResolver(ctrl)
		groups.EXPECT().Get(t.Context(), "ghost").Return(nil, errors.New("not found"))

		sut := authz.NewScope(authz.NewPDP(enf), pap, groups)

		err := sut.RequireWriteUser(t.Context(), "alice", "bob")
		require.ErrorContains(t, err, "find group by name ghost: not found")
	})
}

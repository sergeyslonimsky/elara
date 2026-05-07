package auth_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/auth"
	"github.com/sergeyslonimsky/elara/internal/domain"
	authuc "github.com/sergeyslonimsky/elara/internal/usecase/auth"
	auth_mock "github.com/sergeyslonimsky/elara/internal/usecase/auth/mocks"
)

// groupGetterUpdater wraps separate getter and updater mocks into a single type
// that satisfies the anonymous interface{ groupGetter; groupUpdater }.
type groupGetterUpdater struct {
	*auth_mock.MockgroupGetter
	*auth_mock.MockgroupUpdater
}

func TestCreateGroupUseCase_Execute(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		mockFunc func(context.Context, *gomock.Controller) (*authuc.CreateGroupUseCase, context.Context)
		errIs    error
		wantErr  string
	}{
		{
			name:  "success",
			input: "my-group",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*authuc.CreateGroupUseCase, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "admin@example.com"})
				enforcer := auth_mock.NewMockgroupEnforcer(ctrl)
				creator := auth_mock.NewMockgroupCreator(ctrl)

				enforcer.EXPECT().
					Enforce("admin@example.com", auth.ObjectAll, "group", auth.ActionWrite).
					Return(true, nil)
				creator.EXPECT().Create(ctx, gomock.Any()).Return(nil)

				return authuc.NewCreateGroupUseCase(enforcer, creator), ctx
			},
		},
		{
			name:  "repo error",
			input: "fail-group",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*authuc.CreateGroupUseCase, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "admin@example.com"})
				enforcer := auth_mock.NewMockgroupEnforcer(ctrl)
				creator := auth_mock.NewMockgroupCreator(ctrl)

				enforcer.EXPECT().Enforce(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(true, nil)
				creator.EXPECT().Create(ctx, gomock.Any()).Return(errors.New("db error"))

				return authuc.NewCreateGroupUseCase(enforcer, creator), ctx
			},
			wantErr: "create group",
		},
		{
			name: "unauthorized",
			mockFunc: func(ctx context.Context, _ *gomock.Controller) (*authuc.CreateGroupUseCase, context.Context) {
				return authuc.NewCreateGroupUseCase(nil, nil), ctx
			},
			errIs: domain.ErrUnauthorized,
		},
		{
			name: "forbidden",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*authuc.CreateGroupUseCase, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "user@example.com"})
				enforcer := auth_mock.NewMockgroupEnforcer(ctrl)
				enforcer.EXPECT().
					Enforce("user@example.com", auth.ObjectAll, "group", auth.ActionWrite).
					Return(false, nil)

				return authuc.NewCreateGroupUseCase(enforcer, nil), ctx
			},
			errIs: domain.ErrForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			sut, ctx := tt.mockFunc(t.Context(), ctrl)

			got, err := sut.Execute(ctx, tt.input)

			if tt.errIs != nil {
				require.ErrorIs(t, err, tt.errIs)

				return
			}
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.input, got.Name)
			assert.NotEmpty(t, got.ID)
		})
	}
}

func TestGetGroupUseCase_Execute(t *testing.T) {
	t.Parallel()

	groupID := "g1"

	tests := []struct {
		name     string
		mockFunc func(context.Context, *gomock.Controller) (*authuc.GetGroupUseCase, context.Context)
		errIs    error
		wantErr  string
	}{
		{
			name: "success",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*authuc.GetGroupUseCase, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "admin@example.com"})
				enforcer := auth_mock.NewMockgroupEnforcer(ctrl)
				getter := auth_mock.NewMockgroupGetter(ctrl)

				enforcer.EXPECT().
					Enforce("admin@example.com", auth.ObjectAll, "group", auth.ActionRead).
					Return(true, nil)
				getter.EXPECT().
					Get(ctx, groupID).
					Return(&domain.Group{ID: groupID, Name: "test", Members: []string{}}, nil)

				return authuc.NewGetGroupUseCase(enforcer, getter), ctx
			},
		},
		{
			name: "not found",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*authuc.GetGroupUseCase, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "admin@example.com"})
				enforcer := auth_mock.NewMockgroupEnforcer(ctrl)
				getter := auth_mock.NewMockgroupGetter(ctrl)

				enforcer.EXPECT().Enforce(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(true, nil)
				getter.EXPECT().Get(ctx, groupID).Return(nil, domain.NewNotFoundError("group", groupID))

				return authuc.NewGetGroupUseCase(enforcer, getter), ctx
			},
			errIs:   domain.ErrNotFound,
			wantErr: "get group",
		},
		{
			name: "unauthorized",
			mockFunc: func(ctx context.Context, _ *gomock.Controller) (*authuc.GetGroupUseCase, context.Context) {
				return authuc.NewGetGroupUseCase(nil, nil), ctx
			},
			errIs: domain.ErrUnauthorized,
		},
		{
			name: "forbidden",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*authuc.GetGroupUseCase, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "user@example.com"})
				enforcer := auth_mock.NewMockgroupEnforcer(ctrl)
				enforcer.EXPECT().
					Enforce("user@example.com", auth.ObjectAll, "group", auth.ActionRead).
					Return(false, nil)

				return authuc.NewGetGroupUseCase(enforcer, nil), ctx
			},
			errIs: domain.ErrForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			sut, ctx := tt.mockFunc(t.Context(), ctrl)

			got, err := sut.Execute(ctx, groupID)

			if tt.errIs != nil {
				require.ErrorIs(t, err, tt.errIs)

				return
			}
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, groupID, got.ID)
		})
	}
}

func TestUpdateGroupUseCase_Execute(t *testing.T) {
	t.Parallel()

	groupID := "g1"
	oldName := "old-name"
	newName := "new-name"

	tests := []struct {
		name     string
		newName  string
		mockFunc func(context.Context, *gomock.Controller) (*authuc.UpdateGroupUseCase, context.Context)
		wantErr  string
	}{
		{
			name:    "name unchanged -> no Casbin writes",
			newName: oldName,
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*authuc.UpdateGroupUseCase, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "admin@example.com"})
				enforcer := auth_mock.NewMockgroupEnforcer(ctrl)
				sync := auth_mock.NewMockgroupSyncEnforcer(ctrl)
				getter := auth_mock.NewMockgroupGetter(ctrl)
				updater := auth_mock.NewMockgroupUpdater(ctrl)
				repo := &groupGetterUpdater{getter, updater}

				enforcer.EXPECT().Enforce(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(true, nil)
				getter.EXPECT().
					Get(ctx, groupID).
					Return(&domain.Group{ID: groupID, Name: oldName, Members: []string{}}, nil)
				updater.EXPECT().Update(ctx, gomock.Any()).Return(nil)

				return authuc.NewUpdateGroupUseCase(enforcer, sync, repo), ctx
			},
		},
		{
			name:    "name changed, group has 2 role assignments -> Add/Remove called twice",
			newName: newName,
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*authuc.UpdateGroupUseCase, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "admin@example.com"})
				enforcer := auth_mock.NewMockgroupEnforcer(ctrl)
				sync := auth_mock.NewMockgroupSyncEnforcer(ctrl)
				getter := auth_mock.NewMockgroupGetter(ctrl)
				updater := auth_mock.NewMockgroupUpdater(ctrl)
				repo := &groupGetterUpdater{getter, updater}

				enforcer.EXPECT().Enforce(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(true, nil)
				getter.EXPECT().
					Get(ctx, groupID).
					Return(&domain.Group{ID: groupID, Name: oldName, Members: []string{}}, nil)
				updater.EXPECT().Update(ctx, gomock.Any()).Return(nil)

				rules := [][]string{
					{oldName, "role:admin", "ns1"},
					{oldName, "role:writer", "ns2"},
				}
				sync.EXPECT().GetRulesForSubject(oldName).Return(rules)
				sync.EXPECT().AddRoleForUser(newName, "role:admin", "ns1").Return(nil)
				sync.EXPECT().RemoveRoleForUser(oldName, "role:admin", "ns1").Return(nil)
				sync.EXPECT().AddRoleForUser(newName, "role:writer", "ns2").Return(nil)
				sync.EXPECT().RemoveRoleForUser(oldName, "role:writer", "ns2").Return(nil)

				return authuc.NewUpdateGroupUseCase(enforcer, sync, repo), ctx
			},
		},
		{
			name:    "name changed, group has members -> Add/Remove for each member+rule combo",
			newName: newName,
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*authuc.UpdateGroupUseCase, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "admin@example.com"})
				enforcer := auth_mock.NewMockgroupEnforcer(ctrl)
				sync := auth_mock.NewMockgroupSyncEnforcer(ctrl)
				getter := auth_mock.NewMockgroupGetter(ctrl)
				updater := auth_mock.NewMockgroupUpdater(ctrl)
				repo := &groupGetterUpdater{getter, updater}

				enforcer.EXPECT().Enforce(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(true, nil)
				getter.EXPECT().Get(ctx, groupID).Return(&domain.Group{
					ID: groupID, Name: oldName, Members: []string{"user1@example.com"},
				}, nil)
				updater.EXPECT().Update(ctx, gomock.Any()).Return(nil)

				rules := [][]string{{oldName, "role:admin", "ns1"}}
				sync.EXPECT().GetRulesForSubject(oldName).Return(rules)

				sync.EXPECT().AddRoleForUser(newName, "role:admin", "ns1").Return(nil)
				sync.EXPECT().RemoveRoleForUser(oldName, "role:admin", "ns1").Return(nil)

				sync.EXPECT().AddRoleForUser("user1@example.com", newName, "ns1").Return(nil)
				sync.EXPECT().RemoveRoleForUser("user1@example.com", oldName, "ns1").Return(nil)

				return authuc.NewUpdateGroupUseCase(enforcer, sync, repo), ctx
			},
		},
		{
			name:    "group not found",
			newName: newName,
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*authuc.UpdateGroupUseCase, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "admin@example.com"})
				enforcer := auth_mock.NewMockgroupEnforcer(ctrl)
				getter := auth_mock.NewMockgroupGetter(ctrl)

				enforcer.EXPECT().Enforce(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(true, nil)
				getter.EXPECT().Get(ctx, groupID).Return(nil, domain.NewNotFoundError("group", groupID))

				return authuc.NewUpdateGroupUseCase(enforcer, nil, &groupGetterUpdater{MockgroupGetter: getter}), ctx
			},
			wantErr: "get group",
		},
		{
			name:    "update fails -> Casbin untouched",
			newName: newName,
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*authuc.UpdateGroupUseCase, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "admin@example.com"})
				enforcer := auth_mock.NewMockgroupEnforcer(ctrl)
				sync := auth_mock.NewMockgroupSyncEnforcer(ctrl)
				getter := auth_mock.NewMockgroupGetter(ctrl)
				updater := auth_mock.NewMockgroupUpdater(ctrl)
				repo := &groupGetterUpdater{getter, updater}

				enforcer.EXPECT().Enforce(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(true, nil)
				getter.EXPECT().
					Get(ctx, groupID).
					Return(&domain.Group{ID: groupID, Name: oldName, Members: []string{}}, nil)
				updater.EXPECT().Update(ctx, gomock.Any()).Return(errors.New("update failed"))

				return authuc.NewUpdateGroupUseCase(enforcer, sync, repo), ctx
			},
			wantErr: "update group",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			sut, ctx := tt.mockFunc(t.Context(), ctrl)

			got, err := sut.Execute(ctx, groupID, tt.newName)

			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.newName, got.Name)
		})
	}
}

func TestAddMemberUseCase_Execute(t *testing.T) {
	t.Parallel()

	groupID := "g1"
	groupName := "devops"
	email := "user@example.com"

	tests := []struct {
		name     string
		mockFunc func(context.Context, *gomock.Controller) (*authuc.AddMemberUseCase, context.Context)
		errIs    error
		wantErr  string
	}{
		{
			name: "group has 2 role assignments -> AddRoleForUser called twice",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*authuc.AddMemberUseCase, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "admin@example.com"})

				mockEnforcer := auth_mock.NewMockgroupEnforcer(ctrl)
				mockSync := auth_mock.NewMockgroupSyncEnforcer(ctrl)
				mockGetter := auth_mock.NewMockgroupGetter(ctrl)
				mockUpdater := auth_mock.NewMockgroupUpdater(ctrl)
				repo := &groupGetterUpdater{mockGetter, mockUpdater}

				mockEnforcer.EXPECT().
					Enforce("admin@example.com", auth.ObjectAll, "group", auth.ActionWrite).
					Return(true, nil)
				mockGetter.EXPECT().
					Get(ctx, groupID).
					Return(&domain.Group{ID: groupID, Name: groupName, Members: []string{}}, nil)
				mockUpdater.EXPECT().Update(ctx, gomock.Any()).Return(nil)

				rules := [][]string{
					{groupName, "role:admin", "ns1"},
					{groupName, "role:writer", "ns2"},
				}
				mockSync.EXPECT().GetRulesForSubject(groupName).Return(rules)
				mockSync.EXPECT().AddRoleForUser(email, groupName, "ns1").Return(nil)
				mockSync.EXPECT().AddRoleForUser(email, groupName, "ns2").Return(nil)

				return authuc.NewAddMemberUseCase(mockEnforcer, mockSync, repo), ctx
			},
		},
		{
			name: "group has no role assignments -> AddRoleForUser never called",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*authuc.AddMemberUseCase, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "admin@example.com"})

				mockEnforcer := auth_mock.NewMockgroupEnforcer(ctrl)
				mockSync := auth_mock.NewMockgroupSyncEnforcer(ctrl)
				mockGetter := auth_mock.NewMockgroupGetter(ctrl)
				mockUpdater := auth_mock.NewMockgroupUpdater(ctrl)
				repo := &groupGetterUpdater{mockGetter, mockUpdater}

				mockEnforcer.EXPECT().
					Enforce("admin@example.com", auth.ObjectAll, "group", auth.ActionWrite).
					Return(true, nil)
				mockGetter.EXPECT().
					Get(ctx, groupID).
					Return(&domain.Group{ID: groupID, Name: groupName, Members: []string{}}, nil)
				mockUpdater.EXPECT().Update(ctx, gomock.Any()).Return(nil)

				mockSync.EXPECT().GetRulesForSubject(groupName).Return(nil)

				return authuc.NewAddMemberUseCase(mockEnforcer, mockSync, repo), ctx
			},
		},
		{
			name: "group not found",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*authuc.AddMemberUseCase, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "admin@example.com"})
				mockEnforcer := auth_mock.NewMockgroupEnforcer(ctrl)
				mockGetter := auth_mock.NewMockgroupGetter(ctrl)

				mockEnforcer.EXPECT().Enforce(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(true, nil)
				mockGetter.EXPECT().Get(ctx, groupID).Return(nil, domain.NewNotFoundError("group", groupID))

				return authuc.NewAddMemberUseCase(
					mockEnforcer,
					nil,
					&groupGetterUpdater{MockgroupGetter: mockGetter},
				), ctx
			},
			wantErr: "get group",
		},
		{
			name: "duplicate member returns error",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*authuc.AddMemberUseCase, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "admin@example.com"})
				mockEnforcer := auth_mock.NewMockgroupEnforcer(ctrl)
				mockGetter := auth_mock.NewMockgroupGetter(ctrl)

				mockEnforcer.EXPECT().Enforce(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(true, nil)
				mockGetter.EXPECT().Get(ctx, groupID).Return(&domain.Group{
					ID: groupID, Name: groupName, Members: []string{email},
				}, nil)

				return authuc.NewAddMemberUseCase(
					mockEnforcer,
					nil,
					&groupGetterUpdater{MockgroupGetter: mockGetter},
				), ctx
			},
			wantErr: "add member",
		},
		{
			name: "update fails",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*authuc.AddMemberUseCase, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "admin@example.com"})
				mockEnforcer := auth_mock.NewMockgroupEnforcer(ctrl)
				mockGetter := auth_mock.NewMockgroupGetter(ctrl)
				mockUpdater := auth_mock.NewMockgroupUpdater(ctrl)
				repo := &groupGetterUpdater{mockGetter, mockUpdater}

				mockEnforcer.EXPECT().Enforce(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(true, nil)
				mockGetter.EXPECT().Get(ctx, groupID).Return(&domain.Group{
					ID: groupID, Name: groupName, Members: []string{},
				}, nil)
				mockUpdater.EXPECT().Update(ctx, gomock.Any()).Return(errors.New("update failed"))

				return authuc.NewAddMemberUseCase(mockEnforcer, nil, repo), ctx
			},
			wantErr: "update group",
		},
		{
			name: "Casbin sync error -> Execute returns error",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*authuc.AddMemberUseCase, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "admin@example.com"})

				mockEnforcer := auth_mock.NewMockgroupEnforcer(ctrl)
				mockSync := auth_mock.NewMockgroupSyncEnforcer(ctrl)
				mockGetter := auth_mock.NewMockgroupGetter(ctrl)
				mockUpdater := auth_mock.NewMockgroupUpdater(ctrl)
				repo := &groupGetterUpdater{mockGetter, mockUpdater}

				mockEnforcer.EXPECT().
					Enforce("admin@example.com", auth.ObjectAll, "group", auth.ActionWrite).
					Return(true, nil)
				mockGetter.EXPECT().
					Get(ctx, groupID).
					Return(&domain.Group{ID: groupID, Name: groupName, Members: []string{}}, nil)
				mockUpdater.EXPECT().Update(ctx, gomock.Any()).Return(nil)

				rules := [][]string{{groupName, "role:admin", "ns1"}}
				mockSync.EXPECT().GetRulesForSubject(groupName).Return(rules)
				mockSync.EXPECT().AddRoleForUser(email, groupName, "ns1").Return(errors.New("sync failed"))

				return authuc.NewAddMemberUseCase(mockEnforcer, mockSync, repo), ctx
			},
			wantErr: "sync member role",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			sut, ctx := tt.mockFunc(t.Context(), ctrl)

			got, err := sut.Execute(ctx, groupID, email)

			if tt.errIs != nil {
				require.ErrorIs(t, err, tt.errIs)

				return
			}
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)

				return
			}
			require.NoError(t, err)
			assert.Contains(t, got.Members, email)
		})
	}
}

func TestRemoveMemberUseCase_Execute(t *testing.T) {
	t.Parallel()

	groupID := "g1"
	groupName := "devops"
	email := "user@example.com"

	tests := []struct {
		name     string
		mockFunc func(context.Context, *gomock.Controller) (*authuc.RemoveMemberUseCase, context.Context)
		errIs    error
		wantErr  string
	}{
		{
			name: "group has 2 role assignments -> RemoveRoleForUser called twice",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*authuc.RemoveMemberUseCase, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "admin@example.com"})

				mockEnforcer := auth_mock.NewMockgroupEnforcer(ctrl)
				mockSync := auth_mock.NewMockgroupSyncEnforcer(ctrl)
				mockGetter := auth_mock.NewMockgroupGetter(ctrl)
				mockUpdater := auth_mock.NewMockgroupUpdater(ctrl)
				repo := &groupGetterUpdater{mockGetter, mockUpdater}

				mockEnforcer.EXPECT().
					Enforce("admin@example.com", auth.ObjectAll, "group", auth.ActionWrite).
					Return(true, nil)
				mockGetter.EXPECT().
					Get(ctx, groupID).
					Return(&domain.Group{ID: groupID, Name: groupName, Members: []string{email}}, nil)
				mockUpdater.EXPECT().Update(ctx, gomock.Any()).Return(nil)

				rules := [][]string{
					{groupName, "role:admin", "ns1"},
					{groupName, "role:writer", "ns2"},
				}
				mockSync.EXPECT().GetRulesForSubject(groupName).Return(rules)
				mockSync.EXPECT().RemoveRoleForUser(email, groupName, "ns1").Return(nil)
				mockSync.EXPECT().RemoveRoleForUser(email, groupName, "ns2").Return(nil)

				return authuc.NewRemoveMemberUseCase(mockEnforcer, mockSync, repo), ctx
			},
		},
		{
			name: "group has no role assignments -> RemoveRoleForUser never called",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*authuc.RemoveMemberUseCase, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "admin@example.com"})

				mockEnforcer := auth_mock.NewMockgroupEnforcer(ctrl)
				mockSync := auth_mock.NewMockgroupSyncEnforcer(ctrl)
				mockGetter := auth_mock.NewMockgroupGetter(ctrl)
				mockUpdater := auth_mock.NewMockgroupUpdater(ctrl)
				repo := &groupGetterUpdater{mockGetter, mockUpdater}

				mockEnforcer.EXPECT().
					Enforce("admin@example.com", auth.ObjectAll, "group", auth.ActionWrite).
					Return(true, nil)
				mockGetter.EXPECT().
					Get(ctx, groupID).
					Return(&domain.Group{ID: groupID, Name: groupName, Members: []string{email}}, nil)
				mockUpdater.EXPECT().Update(ctx, gomock.Any()).Return(nil)

				mockSync.EXPECT().GetRulesForSubject(groupName).Return(nil)

				return authuc.NewRemoveMemberUseCase(mockEnforcer, mockSync, repo), ctx
			},
		},
		{
			name: "member not found returns error",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*authuc.RemoveMemberUseCase, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "admin@example.com"})
				mockEnforcer := auth_mock.NewMockgroupEnforcer(ctrl)
				mockGetter := auth_mock.NewMockgroupGetter(ctrl)

				mockEnforcer.EXPECT().Enforce(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(true, nil)
				mockGetter.EXPECT().Get(ctx, groupID).Return(&domain.Group{
					ID: groupID, Name: groupName, Members: []string{},
				}, nil)

				return authuc.NewRemoveMemberUseCase(
					mockEnforcer,
					nil,
					&groupGetterUpdater{MockgroupGetter: mockGetter},
				), ctx
			},
			wantErr: "remove member",
		},
		{
			name: "group not found",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*authuc.RemoveMemberUseCase, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "admin@example.com"})
				mockEnforcer := auth_mock.NewMockgroupEnforcer(ctrl)
				mockGetter := auth_mock.NewMockgroupGetter(ctrl)

				mockEnforcer.EXPECT().Enforce(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(true, nil)
				mockGetter.EXPECT().Get(ctx, groupID).Return(nil, domain.NewNotFoundError("group", groupID))

				return authuc.NewRemoveMemberUseCase(
					mockEnforcer,
					nil,
					&groupGetterUpdater{MockgroupGetter: mockGetter},
				), ctx
			},
			wantErr: "get group",
		},
		{
			name: "update fails",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*authuc.RemoveMemberUseCase, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "admin@example.com"})
				mockEnforcer := auth_mock.NewMockgroupEnforcer(ctrl)
				mockGetter := auth_mock.NewMockgroupGetter(ctrl)
				mockUpdater := auth_mock.NewMockgroupUpdater(ctrl)
				repo := &groupGetterUpdater{mockGetter, mockUpdater}

				mockEnforcer.EXPECT().Enforce(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(true, nil)
				mockGetter.EXPECT().Get(ctx, groupID).Return(&domain.Group{
					ID: groupID, Name: groupName, Members: []string{email},
				}, nil)
				mockUpdater.EXPECT().Update(ctx, gomock.Any()).Return(errors.New("update failed"))

				return authuc.NewRemoveMemberUseCase(mockEnforcer, nil, repo), ctx
			},
			wantErr: "update group",
		},
		{
			name: "Casbin sync error -> Execute returns error",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*authuc.RemoveMemberUseCase, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "admin@example.com"})

				mockEnforcer := auth_mock.NewMockgroupEnforcer(ctrl)
				mockSync := auth_mock.NewMockgroupSyncEnforcer(ctrl)
				mockGetter := auth_mock.NewMockgroupGetter(ctrl)
				mockUpdater := auth_mock.NewMockgroupUpdater(ctrl)
				repo := &groupGetterUpdater{mockGetter, mockUpdater}

				mockEnforcer.EXPECT().
					Enforce("admin@example.com", auth.ObjectAll, "group", auth.ActionWrite).
					Return(true, nil)
				mockGetter.EXPECT().
					Get(ctx, groupID).
					Return(&domain.Group{ID: groupID, Name: groupName, Members: []string{email}}, nil)
				mockUpdater.EXPECT().Update(ctx, gomock.Any()).Return(nil)

				rules := [][]string{{groupName, "role:admin", "ns1"}}
				mockSync.EXPECT().GetRulesForSubject(groupName).Return(rules)
				mockSync.EXPECT().RemoveRoleForUser(email, groupName, "ns1").Return(errors.New("sync failed"))

				return authuc.NewRemoveMemberUseCase(mockEnforcer, mockSync, repo), ctx
			},
			wantErr: "sync remove member role",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			sut, ctx := tt.mockFunc(t.Context(), ctrl)

			got, err := sut.Execute(ctx, groupID, email)

			if tt.errIs != nil {
				require.ErrorIs(t, err, tt.errIs)

				return
			}
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)

				return
			}
			require.NoError(t, err)
			assert.NotContains(t, got.Members, email)
		})
	}
}

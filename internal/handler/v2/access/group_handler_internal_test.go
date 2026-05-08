package access

import (
	"errors"
	"slices"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/domain"
	accessv1 "github.com/sergeyslonimsky/elara/internal/proto/elara/access/v1"
	authuc "github.com/sergeyslonimsky/elara/internal/usecase/auth"
	auth_mock "github.com/sergeyslonimsky/elara/internal/usecase/auth/mocks"
)

// groupGetUpdater combines getter and updater mocks for use cases that require both.
type groupGetUpdater struct {
	*auth_mock.MockgroupGetter
	*auth_mock.MockgroupUpdater
}

// groupGetDeleter combines getter and deleter mocks for DeleteGroupUseCase.
type groupGetDeleter struct {
	*auth_mock.MockgroupGetter
	*auth_mock.MockgroupDeleter
}

// noopGroupSyncEnforcer is a no-op groupSyncEnforcer for handler tests that don't exercise Casbin sync.
type noopGroupSyncEnforcer struct{}

func (noopGroupSyncEnforcer) AddRoleForUser(_, _, _ string) error    { return nil }
func (noopGroupSyncEnforcer) RemoveRoleForUser(_, _, _ string) error { return nil }
func (noopGroupSyncEnforcer) GetRulesForSubject(_ string) [][]string { return nil }

func TestGroupHandler_CreateGroup(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		repoErr error
		wantErr bool
	}{
		{
			name:  "creates group",
			input: "my-group",
		},
		{
			name:    "storage error propagated",
			input:   "bad-group",
			repoErr: errors.New("storage error"),
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			creator := auth_mock.NewMockgroupCreator(ctrl)
			creator.EXPECT().Create(gomock.Any(), gomock.Any()).Return(tc.repoErr)

			h := NewGroupHandler(
				authuc.NewCreateGroupUseCase(allowAllEnforcer{}, creator),
				nil, nil, nil, nil, nil, nil,
			)

			resp, err := h.CreateGroup(
				testCtx(),
				connect.NewRequest(&accessv1.CreateGroupRequest{Name: tc.input}),
			)

			if tc.wantErr {
				require.Error(t, err)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.input, resp.Msg.GetGroup().GetName())
		})
	}
}

func TestGroupHandler_GetGroup(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		id       string
		group    *domain.Group
		repoErr  error
		wantErr  bool
		wantCode connect.Code
	}{
		{
			name:  "returns group",
			id:    "g1",
			group: &domain.Group{ID: "g1", Name: "admins"},
		},
		{
			name:     "not found",
			id:       "missing",
			repoErr:  domain.NewNotFoundError("group", "missing"),
			wantErr:  true,
			wantCode: connect.CodeNotFound,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			getter := auth_mock.NewMockgroupGetter(ctrl)
			getter.EXPECT().Get(gomock.Any(), tc.id).Return(tc.group, tc.repoErr)

			h := NewGroupHandler(
				nil,
				authuc.NewGetGroupUseCase(allowAllEnforcer{}, getter),
				nil, nil, nil, nil, nil,
			)

			resp, err := h.GetGroup(testCtx(), connect.NewRequest(&accessv1.GetGroupRequest{Id: tc.id}))

			if tc.wantErr {
				require.Error(t, err)
				assert.Equal(t, tc.wantCode, connect.CodeOf(err))

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.id, resp.Msg.GetGroup().GetId())
		})
	}
}

func TestGroupHandler_UpdateGroup(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		id        string
		newName   string
		group     *domain.Group
		getErr    error
		updateErr error
		wantErr   bool
	}{
		{
			name:    "updates group name",
			id:      "g1",
			newName: "new-name",
			group:   &domain.Group{ID: "g1", Name: "old-name"},
		},
		{
			name:    "not found returns error",
			id:      "missing",
			newName: "new-name",
			getErr:  domain.NewNotFoundError("group", "missing"),
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			getter := auth_mock.NewMockgroupGetter(ctrl)
			updater := auth_mock.NewMockgroupUpdater(ctrl)
			repo := &groupGetUpdater{getter, updater}

			getter.EXPECT().Get(gomock.Any(), tc.id).Return(tc.group, tc.getErr)
			if tc.getErr == nil {
				updater.EXPECT().Update(gomock.Any(), gomock.Any()).Return(tc.updateErr)
			}

			h := NewGroupHandler(
				nil, nil,
				authuc.NewUpdateGroupUseCase(allowAllEnforcer{}, noopGroupSyncEnforcer{}, repo),
				nil, nil, nil, nil,
			)

			resp, err := h.UpdateGroup(testCtx(), connect.NewRequest(&accessv1.UpdateGroupRequest{
				Id:   tc.id,
				Name: tc.newName,
			}))

			if tc.wantErr {
				require.Error(t, err)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.newName, resp.Msg.GetGroup().GetName())
		})
	}
}

func TestGroupHandler_DeleteGroup(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		id      string
		repoErr error
		wantErr bool
	}{
		{
			name: "deletes group",
			id:   "g1",
		},
		{
			name:    "not found returns error",
			id:      "missing",
			repoErr: domain.NewNotFoundError("group", "missing"),
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			getter := auth_mock.NewMockgroupGetter(ctrl)
			deleter := auth_mock.NewMockgroupDeleter(ctrl)
			repo := &groupGetDeleter{getter, deleter}

			getter.EXPECT().Get(gomock.Any(), tc.id).Return(
				&domain.Group{ID: tc.id, Name: "some-group", Members: []string{}},
				tc.repoErr,
			)
			if tc.repoErr == nil {
				deleter.EXPECT().Delete(gomock.Any(), tc.id).Return(nil)
			}

			h := NewGroupHandler(
				nil, nil, nil,
				authuc.NewDeleteGroupUseCase(allowAllEnforcer{}, noopGroupSyncEnforcer{}, repo),
				nil, nil, nil,
			)

			_, err := h.DeleteGroup(testCtx(), connect.NewRequest(&accessv1.DeleteGroupRequest{Id: tc.id}))

			if tc.wantErr {
				require.Error(t, err)

				return
			}

			require.NoError(t, err)
		})
	}
}

func TestGroupHandler_ListGroups(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		groups  []*domain.Group
		repoErr error
		wantLen int
		wantErr bool
	}{
		{
			name:    "returns all groups",
			groups:  []*domain.Group{{ID: "g1", Name: "admins"}, {ID: "g2", Name: "devs"}},
			wantLen: 2,
		},
		{
			name:    "returns empty list",
			groups:  []*domain.Group{},
			wantLen: 0,
		},
		{
			name:    "storage error propagated",
			repoErr: errors.New("storage error"),
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			lister := auth_mock.NewMockgroupLister(ctrl)
			lister.EXPECT().List(gomock.Any()).Return(tc.groups, tc.repoErr)

			h := NewGroupHandler(
				nil, nil, nil, nil,
				authuc.NewListGroupsUseCase(allowAllEnforcer{}, lister),
				nil, nil,
			)

			resp, err := h.ListGroups(testCtx(), connect.NewRequest(&accessv1.ListGroupsRequest{}))

			if tc.wantErr {
				require.Error(t, err)

				return
			}

			require.NoError(t, err)
			assert.Len(t, resp.Msg.GetGroups(), tc.wantLen)
		})
	}
}

func TestGroupHandler_AddMember(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		groupID string
		email   string
		group   *domain.Group
		getErr  error
		wantErr bool
	}{
		{
			name:    "adds member to group",
			groupID: "g1",
			email:   "alice@example.com",
			group:   &domain.Group{ID: "g1", Name: "test"},
		},
		{
			name:    "group not found",
			groupID: "missing",
			email:   "alice@example.com",
			getErr:  domain.NewNotFoundError("group", "missing"),
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			getter := auth_mock.NewMockgroupGetter(ctrl)
			updater := auth_mock.NewMockgroupUpdater(ctrl)
			repo := &groupGetUpdater{getter, updater}

			getter.EXPECT().Get(gomock.Any(), tc.groupID).Return(tc.group, tc.getErr)
			if tc.getErr == nil {
				updater.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)
			}

			h := NewGroupHandler(
				nil, nil, nil, nil, nil,
				authuc.NewAddMemberUseCase(allowAllEnforcer{}, noopGroupSyncEnforcer{}, repo),
				nil,
			)

			resp, err := h.AddMember(testCtx(), connect.NewRequest(&accessv1.AddMemberRequest{
				GroupId: tc.groupID,
				Email:   tc.email,
			}))

			if tc.wantErr {
				require.Error(t, err)

				return
			}

			require.NoError(t, err)
			assert.Contains(t, resp.Msg.GetGroup().GetMembers(), tc.email)
		})
	}
}

func TestGroupHandler_RemoveMember(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		groupID string
		email   string
		group   *domain.Group
		getErr  error
		wantErr bool
	}{
		{
			name:    "removes member from group",
			groupID: "g1",
			email:   "alice@example.com",
			group:   &domain.Group{ID: "g1", Name: "test", Members: []string{"alice@example.com"}},
		},
		{
			name:    "member not in group returns error",
			groupID: "g1",
			email:   "ghost@example.com",
			group:   &domain.Group{ID: "g1", Name: "test", Members: []string{}},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			getter := auth_mock.NewMockgroupGetter(ctrl)
			updater := auth_mock.NewMockgroupUpdater(ctrl)
			repo := &groupGetUpdater{getter, updater}

			getter.EXPECT().Get(gomock.Any(), tc.groupID).Return(tc.group, tc.getErr)
			if tc.getErr == nil && tc.group != nil {
				if slices.Contains(tc.group.Members, tc.email) {
					updater.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)
				}
			}

			h := NewGroupHandler(
				nil, nil, nil, nil, nil, nil,
				authuc.NewRemoveMemberUseCase(allowAllEnforcer{}, noopGroupSyncEnforcer{}, repo),
			)

			resp, err := h.RemoveMember(testCtx(), connect.NewRequest(&accessv1.RemoveMemberRequest{
				GroupId: tc.groupID,
				Email:   tc.email,
			}))

			if tc.wantErr {
				require.Error(t, err)

				return
			}

			require.NoError(t, err)
			assert.NotContains(t, resp.Msg.GetGroup().GetMembers(), tc.email)
		})
	}
}

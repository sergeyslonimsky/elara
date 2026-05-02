package v2

import (
	"context"
	"errors"
	"slices"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/domain"
	authv1 "github.com/sergeyslonimsky/elara/internal/proto/elara/auth/v1"
	authuc "github.com/sergeyslonimsky/elara/internal/usecase/auth"
	auth_mock "github.com/sergeyslonimsky/elara/internal/usecase/auth/mocks"
)

// groupGetUpdater combines getter and updater mocks for use cases that require both.
type groupGetUpdater struct {
	*auth_mock.MockgroupGetter
	*auth_mock.MockgroupUpdater
}

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
				authuc.NewCreateGroupUseCase(creator),
				nil, nil, nil, nil, nil, nil,
			)

			resp, err := h.CreateGroup(
				context.Background(),
				connect.NewRequest(&authv1.CreateGroupRequest{Name: tc.input}),
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
				authuc.NewGetGroupUseCase(getter),
				nil, nil, nil, nil, nil,
			)

			resp, err := h.GetGroup(context.Background(), connect.NewRequest(&authv1.GetGroupRequest{Id: tc.id}))

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
				authuc.NewUpdateGroupUseCase(repo),
				nil, nil, nil, nil,
			)

			resp, err := h.UpdateGroup(context.Background(), connect.NewRequest(&authv1.UpdateGroupRequest{
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
			deleter := auth_mock.NewMockgroupDeleter(ctrl)
			deleter.EXPECT().Delete(gomock.Any(), tc.id).Return(tc.repoErr)

			h := NewGroupHandler(
				nil, nil, nil,
				authuc.NewDeleteGroupUseCase(deleter),
				nil, nil, nil,
			)

			_, err := h.DeleteGroup(context.Background(), connect.NewRequest(&authv1.DeleteGroupRequest{Id: tc.id}))

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
				authuc.NewListGroupsUseCase(lister),
				nil, nil,
			)

			resp, err := h.ListGroups(context.Background(), connect.NewRequest(&authv1.ListGroupsRequest{}))

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
				authuc.NewAddMemberUseCase(repo),
				nil,
			)

			resp, err := h.AddMember(context.Background(), connect.NewRequest(&authv1.AddMemberRequest{
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
				// Only expect Update when member exists and can be removed.
				if slices.Contains(tc.group.Members, tc.email) {
					updater.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)
				}
			}

			h := NewGroupHandler(
				nil, nil, nil, nil, nil, nil,
				authuc.NewRemoveMemberUseCase(repo),
			)

			resp, err := h.RemoveMember(context.Background(), connect.NewRequest(&authv1.RemoveMemberRequest{
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

package access

import (
	"errors"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/domain"
	access_mock "github.com/sergeyslonimsky/elara/internal/handler/v2/access/mocks"
	accessv1 "github.com/sergeyslonimsky/elara/internal/proto/elara/access/v1"
)

func TestGroupHandler_CreateGroup(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		ucErr   error
		wantErr bool
	}{
		{
			name:  "creates group",
			input: "my-group",
		},
		{
			name:    "usecase error propagated",
			input:   "bad-group",
			ucErr:   errors.New("storage error"),
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			uc := access_mock.NewMockgroupUsecase(ctrl)
			if tc.ucErr != nil {
				uc.EXPECT().Create(gomock.Any(), tc.input).Return(nil, tc.ucErr)
			} else {
				uc.EXPECT().Create(gomock.Any(), tc.input).
					Return(&domain.Group{Name: tc.input}, nil)
			}

			h := NewGroupHandler(uc)

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
		ucErr    error
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
			ucErr:    domain.NewNotFoundError("group", "missing"),
			wantErr:  true,
			wantCode: connect.CodeNotFound,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			uc := access_mock.NewMockgroupUsecase(ctrl)
			uc.EXPECT().Get(gomock.Any(), tc.id).Return(tc.group, tc.ucErr)

			h := NewGroupHandler(uc)

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
		name    string
		id      string
		newName string
		group   *domain.Group
		ucErr   error
		wantErr bool
	}{
		{
			name:    "updates group name",
			id:      "g1",
			newName: "new-name",
			group:   &domain.Group{ID: "g1", Name: "new-name"},
		},
		{
			name:    "not found returns error",
			id:      "missing",
			newName: "new-name",
			ucErr:   domain.NewNotFoundError("group", "missing"),
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			uc := access_mock.NewMockgroupUsecase(ctrl)
			uc.EXPECT().Update(gomock.Any(), tc.id, tc.newName).Return(tc.group, tc.ucErr)

			h := NewGroupHandler(uc)

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
		ucErr   error
		wantErr bool
	}{
		{
			name: "deletes group",
			id:   "g1",
		},
		{
			name:    "not found returns error",
			id:      "missing",
			ucErr:   domain.NewNotFoundError("group", "missing"),
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			uc := access_mock.NewMockgroupUsecase(ctrl)
			uc.EXPECT().Delete(gomock.Any(), tc.id).Return(tc.ucErr)

			h := NewGroupHandler(uc)

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
		ucErr   error
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
			name:    "usecase error propagated",
			ucErr:   errors.New("storage error"),
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			uc := access_mock.NewMockgroupUsecase(ctrl)
			uc.EXPECT().List(gomock.Any()).Return(tc.groups, tc.ucErr)

			h := NewGroupHandler(uc)

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
		ucErr   error
		wantErr bool
	}{
		{
			name:    "adds member to group",
			groupID: "g1",
			email:   "alice@example.com",
			group:   &domain.Group{ID: "g1", Name: "test", Members: []string{"alice@example.com"}},
		},
		{
			name:    "group not found",
			groupID: "missing",
			email:   "alice@example.com",
			ucErr:   domain.NewNotFoundError("group", "missing"),
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			uc := access_mock.NewMockgroupUsecase(ctrl)
			uc.EXPECT().AddMember(gomock.Any(), tc.groupID, tc.email).Return(tc.group, tc.ucErr)

			h := NewGroupHandler(uc)

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
		ucErr   error
		wantErr bool
	}{
		{
			name:    "removes member from group",
			groupID: "g1",
			email:   "alice@example.com",
			group:   &domain.Group{ID: "g1", Name: "test", Members: []string{}},
		},
		{
			name:    "member not in group returns error",
			groupID: "g1",
			email:   "ghost@example.com",
			ucErr:   errors.New("member not in group"),
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			uc := access_mock.NewMockgroupUsecase(ctrl)
			uc.EXPECT().RemoveMember(gomock.Any(), tc.groupID, tc.email).Return(tc.group, tc.ucErr)

			h := NewGroupHandler(uc)

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

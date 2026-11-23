package token_test

import (
	"errors"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/usecase/token"
)

func TestService_Create(t *testing.T) {
	t.Parallel()

	const callerEmail = "user@example.com"

	tests := []struct {
		name     string
		input    token.CreateInput
		mockFunc func(m mocks)
		errIs    error
		wantErr  string
	}{
		{
			name: "success",
			input: token.CreateInput{
				Name:       "test-token",
				Namespaces: []string{"ns1"},
				Role:       domain.RoleWriter,
			},
			mockFunc: func(m mocks) {
				m.pdp.EXPECT().HasNamespace(testUserID, "ns1", domain.ActionWrite).Return(true)
				m.store.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)
			},
		},
		{
			name: "role-boundary denied: writer token requires config:write on ns",
			input: token.CreateInput{
				Name:       "test-token",
				Namespaces: []string{"ns1"},
				Role:       domain.RoleWriter,
			},
			mockFunc: func(m mocks) {
				m.pdp.EXPECT().HasNamespace(testUserID, "ns1", domain.ActionWrite).Return(false)
			},
			errIs: domain.ErrPermissionEscalation,
		},
		{
			name: "role-boundary: reader token requires config:read on ns",
			input: token.CreateInput{
				Name:       "test-token",
				Namespaces: []string{"ns1"},
				Role:       domain.RoleReader,
			},
			mockFunc: func(m mocks) {
				m.pdp.EXPECT().HasNamespace(testUserID, "ns1", domain.ActionRead).Return(false)
			},
			errIs: domain.ErrPermissionEscalation,
		},
		{
			name: "invalid role",
			input: token.CreateInput{
				Name:       "test-token",
				Namespaces: []string{"ns1"},
				Role:       "bogus",
			},
			mockFunc: func(_ mocks) {},
			wantErr:  "must be reader or writer",
		},
		{
			name: "empty namespaces",
			input: token.CreateInput{
				Name:       "test-token",
				Namespaces: []string{},
			},
			mockFunc: func(_ mocks) {},
			wantErr:  "at least one namespace is required",
		},
		{
			name: "store error",
			input: token.CreateInput{
				Name:       "test-token",
				Namespaces: []string{"ns1"},
				Role:       domain.RoleReader,
			},
			mockFunc: func(m mocks) {
				m.pdp.EXPECT().HasNamespace(testUserID, "ns1", domain.ActionRead).Return(true)
				m.store.EXPECT().Create(gomock.Any(), gomock.Any()).Return(errors.New("db error"))
			},
			wantErr: "create token: db error",
		},
		{
			name: "invalid name prefix",
			input: token.CreateInput{
				Name:       "Test Token",
				Namespaces: []string{"ns1"},
				Role:       domain.RoleReader,
			},
			mockFunc: func(m mocks) {
				m.pdp.EXPECT().HasNamespace(testUserID, "ns1", domain.ActionRead).Return(true)
			},
			wantErr: "validate: validation: name: name prefix may contain only lowercase letters",
		},
		{
			name: "name prefix too long",
			input: token.CreateInput{
				Name:       strings.Repeat("a", domain.MaxTokenNamePrefixLen+1),
				Namespaces: []string{"ns1"},
				Role:       domain.RoleReader,
			},
			mockFunc: func(m mocks) {
				m.pdp.EXPECT().HasNamespace(testUserID, "ns1", domain.ActionRead).Return(true)
			},
			wantErr: "name prefix must be at most 48 characters",
		},
		{
			name: "name prefix at max length is accepted",
			input: token.CreateInput{
				Name:       strings.Repeat("a", domain.MaxTokenNamePrefixLen),
				Namespaces: []string{"ns1"},
				Role:       domain.RoleReader,
			},
			mockFunc: func(m mocks) {
				m.pdp.EXPECT().HasNamespace(testUserID, "ns1", domain.ActionRead).Return(true)
				m.store.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)
			},
		},
		{
			name: "username collision is retried with a new suffix",
			input: token.CreateInput{
				Name:       "ci",
				Namespaces: []string{"ns1"},
				Role:       domain.RoleReader,
			},
			mockFunc: func(m mocks) {
				m.pdp.EXPECT().HasNamespace(testUserID, "ns1", domain.ActionRead).Return(true)
				gomock.InOrder(
					m.store.EXPECT().Create(gomock.Any(), gomock.Any()).Return(domain.ErrTokenUsernameTaken).Times(2),
					m.store.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil),
				)
			},
		},
		{
			name: "username collision on every attempt fails",
			input: token.CreateInput{
				Name:       "ci",
				Namespaces: []string{"ns1"},
				Role:       domain.RoleReader,
			},
			mockFunc: func(m mocks) {
				m.pdp.EXPECT().HasNamespace(testUserID, "ns1", domain.ActionRead).Return(true)
				m.store.EXPECT().Create(gomock.Any(), gomock.Any()).Return(domain.ErrTokenUsernameTaken).Times(5)
			},
			wantErr: "create token: token username already taken",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			svc, m := setupService(t)
			tt.mockFunc(m)

			testToken, rawToken, err := svc.Create(t.Context(), authUser(callerEmail), tt.input)

			if tt.errIs != nil {
				require.ErrorIs(t, err, tt.errIs)

				return
			}
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				assert.Nil(t, testToken)
				assert.Empty(t, rawToken)

				return
			}
			require.NoError(t, err)

			assert.NotEmpty(t, testToken.ID)
			assert.Equal(t, callerEmail, testToken.IssuedBy)
			assert.Equal(t, tt.input.Name, testToken.Name)
			assert.Equal(t, tt.input.Namespaces, testToken.Namespaces)
			assert.Equal(t, tt.input.Role, testToken.Role)
			assert.True(t, strings.HasPrefix(rawToken, "elara_"))
			assert.Regexp(t, "^"+regexp.QuoteMeta(tt.input.Name)+"-[a-z2-7]{6}$", testToken.Username)
			assert.Equal(t, token.HashRawToken(rawToken), testToken.TokenHash)
		})
	}
}

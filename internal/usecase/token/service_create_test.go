package token_test

import (
	"errors"
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
				m.pdp.EXPECT().Has(callerEmail, domain.Permission{
					Object: domain.ObjectNamespace,
					Action: domain.ActionWrite,
					Domain: "ns1",
				}).Return(true)
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
				m.pdp.EXPECT().Has(callerEmail, domain.Permission{
					Object: domain.ObjectNamespace,
					Action: domain.ActionWrite,
					Domain: "ns1",
				}).Return(false)
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
				m.pdp.EXPECT().Has(callerEmail, domain.Permission{
					Object: domain.ObjectNamespace,
					Action: domain.ActionRead,
					Domain: "ns1",
				}).Return(false)
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
				m.pdp.EXPECT().Has(callerEmail, domain.Permission{
					Object: domain.ObjectNamespace,
					Action: domain.ActionRead,
					Domain: "ns1",
				}).Return(true)
				m.store.EXPECT().Create(gomock.Any(), gomock.Any()).Return(errors.New("db error"))
			},
			wantErr: "db error",
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

				return
			}
			require.NoError(t, err)

			assert.NotEmpty(t, testToken.ID)
			assert.Equal(t, callerEmail, testToken.IssuedBy)
			assert.Equal(t, tt.input.Name, testToken.Name)
			assert.Equal(t, tt.input.Namespaces, testToken.Namespaces)
			assert.Equal(t, tt.input.Role, testToken.Role)
			assert.True(t, strings.HasPrefix(rawToken, "elara_"))
		})
	}
}

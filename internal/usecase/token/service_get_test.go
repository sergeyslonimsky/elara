package token_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

func TestService_Get(t *testing.T) {
	t.Parallel()

	testToken := &domain.Token{ID: "t1", IssuedBy: "owner@example.com", Namespaces: []string{"ns1"}}

	tests := []struct {
		name     string
		caller   string
		id       string
		mockFunc func(m mocks)
		errIs    error
		wantErr  string
	}{
		{
			name:   "caller with token read on scoped namespace",
			caller: "user@example.com",
			id:     "t1",
			mockFunc: func(m mocks) {
				m.store.EXPECT().GetByID(gomock.Any(), "t1").Return(testToken, nil)
				m.pdp.EXPECT().Has("user@example.com", domain.Permission{
					Object: domain.ObjectToken,
					Action: domain.ActionRead,
					Domain: "ns1",
				}).Return(true)
			},
		},
		{
			name:   "forbidden when no token read on any scoped namespace",
			caller: "stranger@example.com",
			id:     "t1",
			mockFunc: func(m mocks) {
				m.store.EXPECT().GetByID(gomock.Any(), "t1").Return(testToken, nil)
				m.pdp.EXPECT().Has("stranger@example.com", domain.Permission{
					Object: domain.ObjectToken,
					Action: domain.ActionRead,
					Domain: "ns1",
				}).Return(false)
			},
			errIs: domain.ErrForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			svc, m := setupService(t)
			tt.mockFunc(m)

			got, err := svc.Get(t.Context(), authUser(tt.caller), tt.id)

			if tt.errIs != nil {
				require.ErrorIs(t, err, tt.errIs)

				return
			}
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)

				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.id, got.ID)
		})
	}
}

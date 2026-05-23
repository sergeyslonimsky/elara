package user_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// Authorization `(User, Write, *)` is enforced in the handler (EL-4 M9);
// this test covers only the SetPassword flow.

func TestService_ResetPassword(t *testing.T) {
	t.Parallel()

	const (
		targetEmail = "user@example.com"
		newPassword = "reset-password"
	)

	tests := []struct {
		name     string
		mockFunc func(ctx context.Context, m mocks)
		wantErr  string
	}{
		{
			name: "success",
			mockFunc: func(ctx context.Context, m mocks) {
				m.store.EXPECT().SetPassword(ctx, targetEmail, gomock.Any(), true).Return(nil)
			},
		},
		{
			name: "set password fails",
			mockFunc: func(ctx context.Context, m mocks) {
				m.store.EXPECT().SetPassword(ctx, targetEmail, gomock.Any(), true).Return(assert.AnError)
			},
			wantErr: "set password",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			sut, m, _, _ := setupService(t)
			ctx := t.Context()
			tt.mockFunc(ctx, m)

			err := sut.ResetPassword(ctx, targetEmail, newPassword)

			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)

				return
			}
			require.NoError(t, err)
		})
	}
}

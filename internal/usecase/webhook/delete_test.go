package webhook_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	webhookuc "github.com/sergeyslonimsky/elara/internal/usecase/webhook"
	webhook_mock "github.com/sergeyslonimsky/elara/internal/usecase/webhook/mocks"
)

func TestDeleteUseCase_Execute(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		repoErr        error
		wantErr        bool
		wantHistClears bool
	}{
		{
			name:           "success clears history",
			wantErr:        false,
			wantHistClears: true,
		},
		{
			name:           "repo error propagated",
			repoErr:        errors.New("db failure"),
			wantErr:        true,
			wantHistClears: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			repo := webhook_mock.NewMockwebhookDeleter(ctrl)
			clearer := webhook_mock.NewMockhistoryClearer(ctrl)

			repo.EXPECT().Delete(gomock.Any(), "wh-1").Return(tt.repoErr)

			if tt.wantHistClears {
				clearer.EXPECT().ClearHistory("wh-1")
			}

			uc := webhookuc.NewDeleteUseCase(repo, clearer)
			err := uc.Execute(t.Context(), "wh-1")

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

package webhook_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/domain"
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
			getter := webhook_mock.NewMockdeleteWebhookGetter(ctrl)
			repo := webhook_mock.NewMockwebhookDeleter(ctrl)
			clearer := webhook_mock.NewMockhistoryClearer(ctrl)

			getter.EXPECT().Get(gomock.Any(), "wh-1").Return(&domain.Webhook{ID: "wh-1"}, nil)

			if !tt.wantErr {
				repo.EXPECT().Delete(gomock.Any(), "wh-1").Return(nil)
			} else {
				repo.EXPECT().Delete(gomock.Any(), "wh-1").Return(tt.repoErr)
			}

			if tt.wantHistClears {
				clearer.EXPECT().ClearHistory("wh-1")
			}

			uc := webhookuc.NewDeleteUseCase(allowAllWebhookEnforcer{}, getter, repo, clearer)
			err := uc.Execute(webhookTestCtx(), "wh-1")

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

package webhook_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/domain"
	webhookuc "github.com/sergeyslonimsky/elara/internal/usecase/webhook"
	webhook_mock "github.com/sergeyslonimsky/elara/internal/usecase/webhook/mocks"
)

func TestHistoryUseCase_Execute(t *testing.T) {
	t.Parallel()

	knownID := "wh-known"
	attempts := []domain.DeliveryAttempt{
		{
			AttemptNumber: 1,
			StatusCode:    200,
			LatencyMS:     42,
			Success:       true,
			Timestamp:     time.Now(),
		},
		{
			AttemptNumber: 2,
			StatusCode:    500,
			LatencyMS:     100,
			Error:         "internal server error",
			Success:       false,
			Timestamp:     time.Now(),
		},
	}

	tests := []struct {
		name      string
		webhookID string
		wantLen   int
	}{
		{
			name:      "returns delivery attempts for known webhook ID",
			webhookID: knownID,
			wantLen:   len(attempts),
		},
		{
			name:      "returns empty slice for unknown webhook ID",
			webhookID: "wh-unknown",
			wantLen:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			provider := webhook_mock.NewMockdeliveryHistoryProvider(ctrl)

			if tt.webhookID == knownID {
				provider.EXPECT().GetDeliveryHistory(tt.webhookID).Return(attempts)
			} else {
				provider.EXPECT().GetDeliveryHistory(tt.webhookID).Return([]domain.DeliveryAttempt{})
			}

			uc := webhookuc.NewHistoryUseCase(provider)
			result := uc.Execute(tt.webhookID)

			assert.Len(t, result, tt.wantLen)
		})
	}
}

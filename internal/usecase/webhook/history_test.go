package webhook_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/sergeyslonimsky/elara/internal/domain"
	webhookuc "github.com/sergeyslonimsky/elara/internal/usecase/webhook"
)

type mockHistoryProvider struct {
	history map[string][]domain.DeliveryAttempt
}

func (m *mockHistoryProvider) GetDeliveryHistory(webhookID string) []domain.DeliveryAttempt {
	if attempts, ok := m.history[webhookID]; ok {
		return attempts
	}

	return []domain.DeliveryAttempt{}
}

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

	provider := &mockHistoryProvider{
		history: map[string][]domain.DeliveryAttempt{
			knownID: attempts,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			uc := webhookuc.NewHistoryUseCase(provider)
			result := uc.Execute(tt.webhookID)

			assert.Len(t, result, tt.wantLen)
		})
	}
}

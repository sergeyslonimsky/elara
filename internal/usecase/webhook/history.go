package webhook

import "github.com/sergeyslonimsky/elara/internal/domain"

//go:generate mockgen -destination=mocks/mock_history.go -package=webhook_mock . deliveryHistoryProvider

type deliveryHistoryProvider interface {
	GetDeliveryHistory(webhookID string) []domain.DeliveryAttempt
}

type HistoryUseCase struct {
	dispatcher deliveryHistoryProvider
}

func NewHistoryUseCase(dispatcher deliveryHistoryProvider) *HistoryUseCase {
	return &HistoryUseCase{dispatcher: dispatcher}
}

func (uc *HistoryUseCase) Execute(webhookID string) []domain.DeliveryAttempt {
	return uc.dispatcher.GetDeliveryHistory(webhookID)
}

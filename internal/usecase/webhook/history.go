package webhook

import "github.com/sergeyslonimsky/elara/internal/domain"

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

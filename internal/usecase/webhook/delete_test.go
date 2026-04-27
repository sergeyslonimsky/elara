package webhook_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	webhookuc "github.com/sergeyslonimsky/elara/internal/usecase/webhook"
)

type mockDeleter struct {
	err error
}

func (m *mockDeleter) Delete(_ context.Context, _ string) error {
	return m.err
}

type mockHistoryClearer struct {
	clearedID string
}

func (m *mockHistoryClearer) ClearHistory(webhookID string) {
	m.clearedID = webhookID
}

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

			clearer := &mockHistoryClearer{}
			uc := webhookuc.NewDeleteUseCase(&mockDeleter{err: tt.repoErr}, clearer)

			err := uc.Execute(t.Context(), "wh-1")

			if tt.wantErr {
				require.Error(t, err)
				assert.Empty(t, clearer.clearedID)
			} else {
				require.NoError(t, err)
				assert.Equal(t, "wh-1", clearer.clearedID)
			}
		})
	}
}

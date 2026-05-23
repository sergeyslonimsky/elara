package group_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

func TestService_Get(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		setup   func(t *testing.T, st testStack) string
		assert  func(t *testing.T, got *domain.Group)
		wantErr string
	}{
		{
			name: "success",
			setup: func(t *testing.T, st testStack) string {
				t.Helper()

				created, err := st.svc.Create(t.Context(), adminAuth(), "g1")
				require.NoError(t, err)

				return created.ID
			},
			assert: func(t *testing.T, got *domain.Group) {
				t.Helper()

				assert.Equal(t, "g1", got.Name)
			},
		},
		{
			name: "not found",
			setup: func(_ *testing.T, _ testStack) string {
				return "missing-id"
			},
			wantErr: "get group",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			st := newTestStack(t)
			id := tt.setup(t, st)

			got, err := st.svc.Get(t.Context(), id)
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)

				return
			}
			require.NoError(t, err)
			if tt.assert != nil {
				tt.assert(t, got)
			}
		})
	}
}

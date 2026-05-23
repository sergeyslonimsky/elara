package group_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestService_Create(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		groupName string
		wantErr   string
	}{
		{name: "success", groupName: "test-group"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			st := newTestStack(t)

			got, err := st.svc.Create(t.Context(), adminAuth(), tt.groupName)
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)

				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.groupName, got.Name)
			assert.NotEmpty(t, got.ID)
		})
	}
}

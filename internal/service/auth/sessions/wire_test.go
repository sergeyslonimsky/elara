package sessions_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/auth/sessions"
	sessions_mock "github.com/sergeyslonimsky/elara/internal/service/auth/sessions/mocks"
)

// TestNewService verifies the DI-facing constructor wires the exported
// Repository/EventRepository/Clock dependencies into a usable *Service —
// the generated gomock mocks satisfy the exported interfaces structurally,
// same as the unexported ones service.New consumes.
func TestNewService(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	repo := sessions_mock.NewMocksessionRepository(ctrl)
	events := sessions_mock.NewMocksessionEventRepository(ctrl)
	clk := sessions_mock.NewMockClock(ctrl)

	svc := sessions.NewService(repo, events, clk)
	require.NotNil(t, svc)

	now := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)
	sess := &domain.Session{ID: "sess-1", UserID: "user-1", ExpiresAt: now.Add(time.Hour)}
	repo.EXPECT().Get(gomock.Any(), "sess-1").Return(sess, nil)
	clk.EXPECT().Now().Return(now)

	got, err := svc.Validate(t.Context(), "sess-1")
	require.NoError(t, err)
	assert.Equal(t, sess, got)
}

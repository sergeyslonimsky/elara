package sessions_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/sergeyslonimsky/elara/internal/service/auth/sessions"
)

func TestRealClock_Now(t *testing.T) {
	t.Parallel()

	before := time.Now()
	got := sessions.RealClock{}.Now()
	after := time.Now()

	assert.False(t, got.Before(before))
	assert.False(t, got.After(after))
}

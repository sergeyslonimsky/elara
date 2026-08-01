package webhook

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestCryptoJitter covers the window<1 short-circuit branch that production
// retry delays (5s/30s/120s) never hit, plus a sanity check that a normal
// delay's jittered result stays within its documented ±20% window.
func TestCryptoJitter(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		delay time.Duration
	}{
		{
			// window := int64(delay)/jitterRange*jitterWindowFactor rounds
			// down to 0 for sub-nanosecond-scale delays, taking the
			// window<1 short-circuit that returns delay unchanged.
			name:  "delay too small for a window returns delay unchanged",
			delay: 1,
		},
		{
			name:  "zero delay returns delay unchanged",
			delay: 0,
		},
		{
			name:  "normal delay stays within +-20% window",
			delay: 5 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := cryptoJitter(tt.delay)

			if tt.delay < jitterRange {
				assert.Equal(t, tt.delay, got, "window<1 must return delay unchanged")

				return
			}

			lowerBound := tt.delay * 4 / jitterRange
			upperBound := tt.delay*4/jitterRange + tt.delay*jitterWindowFactor/jitterRange
			assert.GreaterOrEqual(t, got, lowerBound)
			assert.LessOrEqual(t, got, upperBound)
		})
	}
}

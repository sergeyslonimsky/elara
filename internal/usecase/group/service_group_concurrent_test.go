package group_test

import (
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

func TestService_Update_ConcurrentVersionConflict(t *testing.T) {
	t.Parallel()

	sut, _, _, _ := newTestService(t)
	ctx := contextWithAdmin(t.Context())

	// Setup: create a group with version=1
	created, err := sut.Create(ctx, "concurrent-group")
	require.NoError(t, err)

	var wg sync.WaitGroup
	errs := make(chan error, 2)

	// Run two concurrent updates with the same base version
	wg.Add(2)

	go func() {
		defer wg.Done()
		_, err := sut.Update(
			ctx,
			created.ID,
			"concurrent-group",
			"Update A",
			nil,
			[]string{"alice@example.com"},
			created.Version,
		)
		errs <- err
	}()

	go func() {
		defer wg.Done()
		_, err := sut.Update(
			ctx,
			created.ID,
			"concurrent-group",
			"Update B",
			nil,
			[]string{"bob@example.com"},
			created.Version,
		)
		errs <- err
	}()

	wg.Wait()
	close(errs)

	var successCount, conflictCount int
	for err := range errs {
		switch {
		case err == nil:
			successCount++
		case errors.Is(err, domain.ErrVersionConflict):
			conflictCount++
		default:
			t.Errorf("unexpected error: %v", err)
		}
	}

	assert.Equal(t, 1, successCount, "exactly one update should succeed")
	assert.Equal(t, 1, conflictCount, "exactly one update should fail with version conflict")

	// Final verification
	final, err := sut.Get(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, created.Version+1, final.Version, "version should increment exactly once")

	switch final.Description {
	case "Update A":
		assert.Contains(t, final.Members, "alice@example.com")
		assert.NotContains(t, final.Members, "bob@example.com")
	case "Update B":
		assert.Contains(t, final.Members, "bob@example.com")
		assert.NotContains(t, final.Members, "alice@example.com")
	default:
		t.Errorf("unexpected final description: %s", final.Description)
	}
}

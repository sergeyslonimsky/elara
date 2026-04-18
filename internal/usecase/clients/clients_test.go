package clients_test

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/internal/domain"
	clientsuc "github.com/sergeyslonimsky/elara/internal/usecase/clients"
)

type fakeActive struct {
	clients []*domain.Client
	events  map[string][]domain.ClientEvent
}

func (s *fakeActive) ListActive() []*domain.Client { return s.clients }
func (s *fakeActive) Get(id string) *domain.Client {
	for _, c := range s.clients {
		if c.ID == id {
			return c
		}
	}

	return nil
}
func (s *fakeActive) RecentEvents(id string) []domain.ClientEvent { return s.events[id] }
func (s *fakeActive) Subscribe() (<-chan domain.ClientChange, func()) {
	ch := make(chan domain.ClientChange)

	return ch, func() { close(ch) }
}

func (s *fakeActive) SubscribeClient(_ string) (<-chan domain.ClientChange, func()) {
	return s.Subscribe()
}

type fakeHistory struct {
	saved []*domain.Client
	err   error
}

func (h *fakeHistory) List(_ context.Context, limit int) ([]*domain.Client, error) {
	if h.err != nil {
		return nil, h.err
	}
	if limit <= 0 || limit > len(h.saved) {
		limit = len(h.saved)
	}

	return h.saved[:limit], nil
}

func (h *fakeHistory) ListByClient(_ context.Context, name, ns string, limit int) ([]*domain.Client, error) {
	if h.err != nil {
		return nil, h.err
	}

	var out []*domain.Client
	for _, c := range h.saved {
		if c.ClientName == name && c.K8sNamespace == ns {
			out = append(out, c)
		}
	}

	if limit > 0 && limit < len(out) {
		out = out[:limit]
	}

	return out, nil
}

func TestUseCase_ListActive_SortedByConnectedAtAsc(t *testing.T) {
	t.Parallel()

	now := time.Now()
	uc := clientsuc.NewUseCase(&fakeActive{
		clients: []*domain.Client{
			{ID: "b", ConnectedAt: now.Add(time.Second)},
			{ID: "a", ConnectedAt: now},
			{ID: "c", ConnectedAt: now.Add(2 * time.Second)},
		},
	}, &fakeHistory{})

	got := uc.ListActive(context.Background())
	require.Len(t, got, 3)
	assert.Equal(t, "a", got[0].ID)
	assert.Equal(t, "b", got[1].ID)
	assert.Equal(t, "c", got[2].ID)
}

func TestUseCase_Get_Active_ReturnsRecentEvents(t *testing.T) {
	t.Parallel()

	uc := clientsuc.NewUseCase(&fakeActive{
		clients: []*domain.Client{{ID: "x"}},
		events:  map[string][]domain.ClientEvent{"x": {{Method: "Put"}}},
	}, &fakeHistory{})

	c, ev, err := uc.Get(context.Background(), "x")
	require.NoError(t, err)
	require.NotNil(t, c)
	require.Len(t, ev, 1)
	assert.Equal(t, "Put", ev[0].Method)
}

func TestUseCase_Get_FallbackToHistory_WhenNotActive(t *testing.T) {
	t.Parallel()

	d := time.Now()
	uc := clientsuc.NewUseCase(&fakeActive{}, &fakeHistory{
		saved: []*domain.Client{
			{ID: "old", DisconnectedAt: &d},
			{ID: "older", DisconnectedAt: &d},
		},
	})

	c, ev, err := uc.Get(context.Background(), "older")
	require.NoError(t, err)
	require.NotNil(t, c)
	assert.Equal(t, "older", c.ID)
	assert.Nil(t, ev, "history clients have no recent events")
}

func TestUseCase_Get_NotFoundAnywhere(t *testing.T) {
	t.Parallel()

	uc := clientsuc.NewUseCase(&fakeActive{}, &fakeHistory{})

	c, ev, err := uc.Get(context.Background(), "nope")
	require.NoError(t, err)
	assert.Nil(t, c)
	assert.Nil(t, ev)
}

func TestUseCase_Get_PropagatesHistoryError(t *testing.T) {
	t.Parallel()

	uc := clientsuc.NewUseCase(&fakeActive{}, &fakeHistory{err: errors.New("boom")})

	_, _, err := uc.Get(context.Background(), "anything")
	require.Error(t, err)
}

func TestUseCase_ListHistorical_DefaultLimit(t *testing.T) {
	t.Parallel()

	d := time.Now()
	saved := make([]*domain.Client, 200)
	for i := range saved {
		saved[i] = &domain.Client{ID: "x", DisconnectedAt: &d}
	}

	uc := clientsuc.NewUseCase(&fakeActive{}, &fakeHistory{saved: saved})
	got, err := uc.ListHistorical(context.Background(), 0)
	require.NoError(t, err)
	assert.Len(t, got, 100, "default limit applied when 0 passed")
}

func TestUseCase_ListHistorical_RespectsLimit(t *testing.T) {
	t.Parallel()

	d := time.Now()
	saved := []*domain.Client{
		{ID: "a", DisconnectedAt: &d},
		{ID: "b", DisconnectedAt: &d},
		{ID: "c", DisconnectedAt: &d},
	}

	uc := clientsuc.NewUseCase(&fakeActive{}, &fakeHistory{saved: saved})
	got, err := uc.ListHistorical(context.Background(), 2)
	require.NoError(t, err)
	assert.Len(t, got, 2)
}

func TestUseCase_ListSessions(t *testing.T) {
	t.Parallel()

	d := time.Now()
	saved := []*domain.Client{
		{ID: "a", ClientName: "order-service", K8sNamespace: "production", DisconnectedAt: &d},
		{ID: "b", ClientName: "order-service", K8sNamespace: "production", DisconnectedAt: &d},
		{ID: "c", ClientName: "order-service", K8sNamespace: "staging", DisconnectedAt: &d},
		{ID: "d", ClientName: "payment", K8sNamespace: "production", DisconnectedAt: &d},
	}

	t.Run("matches name+namespace", func(t *testing.T) {
		t.Parallel()

		uc := clientsuc.NewUseCase(&fakeActive{}, &fakeHistory{saved: saved})
		got, err := uc.ListSessions(context.Background(), "order-service", "production", "", 0)
		require.NoError(t, err)
		require.Len(t, got, 2)
		assert.ElementsMatch(t, []string{"a", "b"}, []string{got[0].ID, got[1].ID})
	})

	t.Run("excludes currentID", func(t *testing.T) {
		t.Parallel()

		uc := clientsuc.NewUseCase(&fakeActive{}, &fakeHistory{saved: saved})
		got, err := uc.ListSessions(context.Background(), "order-service", "production", "a", 0)
		require.NoError(t, err)
		require.Len(t, got, 1)
		assert.Equal(t, "b", got[0].ID)
	})

	t.Run("empty client_name → no sessions (anonymous can't be correlated)", func(t *testing.T) {
		t.Parallel()

		uc := clientsuc.NewUseCase(&fakeActive{}, &fakeHistory{saved: saved})
		got, err := uc.ListSessions(context.Background(), "", "production", "", 0)
		require.NoError(t, err)
		assert.Empty(t, got)
	})

	t.Run("respects limit after exclusion", func(t *testing.T) {
		t.Parallel()

		// Add more matching to test trimming
		more := slices.Clone(saved)
		more = append(more,
			&domain.Client{ID: "e", ClientName: "order-service", K8sNamespace: "production", DisconnectedAt: &d},
			&domain.Client{ID: "f", ClientName: "order-service", K8sNamespace: "production", DisconnectedAt: &d},
		)
		uc := clientsuc.NewUseCase(&fakeActive{}, &fakeHistory{saved: more})
		got, err := uc.ListSessions(context.Background(), "order-service", "production", "a", 2)
		require.NoError(t, err)
		require.Len(t, got, 2, "limit honored after excluding currentID")
	})
}

func TestUseCase_SubscribeClient_DelegatesToActive(t *testing.T) {
	t.Parallel()

	uc := clientsuc.NewUseCase(&fakeActive{}, &fakeHistory{})

	ch, cleanup := uc.SubscribeClient("any")
	require.NotNil(t, ch)
	cleanup()
}

func TestUseCase_SubscribeChanges_DelegatesToActive(t *testing.T) {
	t.Parallel()

	uc := clientsuc.NewUseCase(&fakeActive{}, &fakeHistory{})

	ch, cleanup := uc.SubscribeChanges()
	require.NotNil(t, ch)
	cleanup()

	_, ok := <-ch
	assert.False(t, ok, "cleanup must close the channel")
}

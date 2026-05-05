package clients_test

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/auth"
	"github.com/sergeyslonimsky/elara/internal/domain"
	clientsuc "github.com/sergeyslonimsky/elara/internal/usecase/clients"
	clients_mock "github.com/sergeyslonimsky/elara/internal/usecase/clients/mocks"
)

type allowAllClientsEnforcer struct{}

func (allowAllClientsEnforcer) Enforce(_, _, _, _ string) (bool, error) { return true, nil }

func clientsTestCtx() context.Context {
	return auth.WithClaims(context.Background(), &auth.Claims{Email: "test@example.com"})
}

func TestUseCase_ListActive_SortedByConnectedAtAsc(t *testing.T) {
	t.Parallel()

	now := time.Now()
	clients := []*domain.Client{
		{ID: "b", ConnectedAt: now.Add(time.Second)},
		{ID: "a", ConnectedAt: now},
		{ID: "c", ConnectedAt: now.Add(2 * time.Second)},
	}

	ctrl := gomock.NewController(t)
	active := clients_mock.NewMockActiveSource(ctrl)
	hist := clients_mock.NewMockHistorySource(ctrl)

	active.EXPECT().ListActive().Return(clients)

	uc := clientsuc.NewUseCase(allowAllClientsEnforcer{}, active, hist)

	got, err := uc.ListActive(clientsTestCtx())
	require.NoError(t, err)
	require.Len(t, got, 3)
	assert.Equal(t, "a", got[0].ID)
	assert.Equal(t, "b", got[1].ID)
	assert.Equal(t, "c", got[2].ID)
}

func TestUseCase_Get_Active_ReturnsRecentEvents(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	active := clients_mock.NewMockActiveSource(ctrl)
	hist := clients_mock.NewMockHistorySource(ctrl)

	active.EXPECT().Get("x").Return(&domain.Client{ID: "x"})
	active.EXPECT().RecentEvents("x").Return([]domain.ClientEvent{{Method: "Put"}})

	uc := clientsuc.NewUseCase(allowAllClientsEnforcer{}, active, hist)

	c, ev, err := uc.Get(clientsTestCtx(), "x")
	require.NoError(t, err)
	require.NotNil(t, c)
	require.Len(t, ev, 1)
	assert.Equal(t, "Put", ev[0].Method)
}

func TestUseCase_Get_FallbackToHistory_WhenNotActive(t *testing.T) {
	t.Parallel()

	d := time.Now()
	historicalClients := []*domain.Client{
		{ID: "old", DisconnectedAt: &d},
		{ID: "older", DisconnectedAt: &d},
	}

	ctrl := gomock.NewController(t)
	active := clients_mock.NewMockActiveSource(ctrl)
	hist := clients_mock.NewMockHistorySource(ctrl)

	active.EXPECT().Get("older").Return(nil)
	hist.EXPECT().List(gomock.Any(), 1000).Return(historicalClients, nil)

	uc := clientsuc.NewUseCase(allowAllClientsEnforcer{}, active, hist)

	c, ev, err := uc.Get(clientsTestCtx(), "older")
	require.NoError(t, err)
	require.NotNil(t, c)
	assert.Equal(t, "older", c.ID)
	assert.Nil(t, ev, "history clients have no recent events")
}

func TestUseCase_Get_NotFoundAnywhere(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	active := clients_mock.NewMockActiveSource(ctrl)
	hist := clients_mock.NewMockHistorySource(ctrl)

	active.EXPECT().Get("nope").Return(nil)
	hist.EXPECT().List(gomock.Any(), 1000).Return([]*domain.Client{}, nil)

	uc := clientsuc.NewUseCase(allowAllClientsEnforcer{}, active, hist)

	c, ev, err := uc.Get(clientsTestCtx(), "nope")
	require.NoError(t, err)
	assert.Nil(t, c)
	assert.Nil(t, ev)
}

func TestUseCase_Get_PropagatesHistoryError(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	active := clients_mock.NewMockActiveSource(ctrl)
	hist := clients_mock.NewMockHistorySource(ctrl)

	active.EXPECT().Get("anything").Return(nil)
	hist.EXPECT().List(gomock.Any(), 1000).Return(nil, errors.New("boom"))

	uc := clientsuc.NewUseCase(allowAllClientsEnforcer{}, active, hist)

	_, _, err := uc.Get(clientsTestCtx(), "anything")
	require.Error(t, err)
}

func TestUseCase_ListHistorical_DefaultLimit(t *testing.T) {
	t.Parallel()

	d := time.Now()
	saved := make([]*domain.Client, 100)
	for i := range saved {
		saved[i] = &domain.Client{ID: "x", DisconnectedAt: &d}
	}

	ctrl := gomock.NewController(t)
	active := clients_mock.NewMockActiveSource(ctrl)
	hist := clients_mock.NewMockHistorySource(ctrl)

	hist.EXPECT().List(gomock.Any(), 100).Return(saved, nil)

	uc := clientsuc.NewUseCase(allowAllClientsEnforcer{}, active, hist)
	got, err := uc.ListHistorical(clientsTestCtx(), 0)
	require.NoError(t, err)
	assert.Len(t, got, 100, "default limit applied when 0 passed")
}

func TestUseCase_ListHistorical_RespectsLimit(t *testing.T) {
	t.Parallel()

	d := time.Now()
	saved := []*domain.Client{
		{ID: "a", DisconnectedAt: &d},
		{ID: "b", DisconnectedAt: &d},
	}

	ctrl := gomock.NewController(t)
	active := clients_mock.NewMockActiveSource(ctrl)
	hist := clients_mock.NewMockHistorySource(ctrl)

	hist.EXPECT().List(gomock.Any(), 2).Return(saved, nil)

	uc := clientsuc.NewUseCase(allowAllClientsEnforcer{}, active, hist)
	got, err := uc.ListHistorical(clientsTestCtx(), 2)
	require.NoError(t, err)
	assert.Len(t, got, 2)
}

func TestUseCase_ListSessions(t *testing.T) {
	t.Parallel()

	d := time.Now()
	orderProduction := []*domain.Client{
		{ID: "a", ClientName: "order-service", K8sNamespace: "production", DisconnectedAt: &d},
		{ID: "b", ClientName: "order-service", K8sNamespace: "production", DisconnectedAt: &d},
	}

	t.Run("matches name+namespace", func(t *testing.T) {
		t.Parallel()

		ctrl := gomock.NewController(t)
		active := clients_mock.NewMockActiveSource(ctrl)
		hist := clients_mock.NewMockHistorySource(ctrl)

		hist.EXPECT().ListByClient(gomock.Any(), "order-service", "production", 51).Return(orderProduction, nil)

		uc := clientsuc.NewUseCase(allowAllClientsEnforcer{}, active, hist)
		got, err := uc.ListSessions(clientsTestCtx(), "order-service", "production", "", 0)
		require.NoError(t, err)
		require.Len(t, got, 2)
		assert.ElementsMatch(t, []string{"a", "b"}, []string{got[0].ID, got[1].ID})
	})

	t.Run("excludes currentID", func(t *testing.T) {
		t.Parallel()

		ctrl := gomock.NewController(t)
		active := clients_mock.NewMockActiveSource(ctrl)
		hist := clients_mock.NewMockHistorySource(ctrl)

		hist.EXPECT().ListByClient(gomock.Any(), "order-service", "production", 51).Return(orderProduction, nil)

		uc := clientsuc.NewUseCase(allowAllClientsEnforcer{}, active, hist)
		got, err := uc.ListSessions(clientsTestCtx(), "order-service", "production", "a", 0)
		require.NoError(t, err)
		require.Len(t, got, 1)
		assert.Equal(t, "b", got[0].ID)
	})

	t.Run("empty client_name → no sessions (anonymous can't be correlated)", func(t *testing.T) {
		t.Parallel()

		ctrl := gomock.NewController(t)
		active := clients_mock.NewMockActiveSource(ctrl)
		hist := clients_mock.NewMockHistorySource(ctrl)
		// ListByClient must not be called for empty client_name.

		uc := clientsuc.NewUseCase(allowAllClientsEnforcer{}, active, hist)
		got, err := uc.ListSessions(clientsTestCtx(), "", "production", "", 0)
		require.NoError(t, err)
		assert.Empty(t, got)
	})

	t.Run("respects limit after exclusion", func(t *testing.T) {
		t.Parallel()

		d := time.Now()
		moreClients := []*domain.Client{
			{ID: "a", ClientName: "order-service", K8sNamespace: "production", DisconnectedAt: &d},
			{ID: "b", ClientName: "order-service", K8sNamespace: "production", DisconnectedAt: &d},
			{ID: "e", ClientName: "order-service", K8sNamespace: "production", DisconnectedAt: &d},
			{ID: "f", ClientName: "order-service", K8sNamespace: "production", DisconnectedAt: &d},
		}
		_ = slices.Clone(moreClients) // ensure slices is used

		ctrl := gomock.NewController(t)
		active := clients_mock.NewMockActiveSource(ctrl)
		hist := clients_mock.NewMockHistorySource(ctrl)

		hist.EXPECT().ListByClient(gomock.Any(), "order-service", "production", 3).Return(moreClients[:3], nil)

		uc := clientsuc.NewUseCase(allowAllClientsEnforcer{}, active, hist)
		got, err := uc.ListSessions(clientsTestCtx(), "order-service", "production", "a", 2)
		require.NoError(t, err)
		require.Len(t, got, 2, "limit honored after excluding currentID")
	})
}

func TestUseCase_SubscribeClient_DelegatesToActive(t *testing.T) {
	t.Parallel()

	ch := make(chan domain.ClientChange)
	cleanup := func() { close(ch) }

	ctrl := gomock.NewController(t)
	active := clients_mock.NewMockActiveSource(ctrl)
	hist := clients_mock.NewMockHistorySource(ctrl)

	var chRecv <-chan domain.ClientChange = ch
	active.EXPECT().SubscribeClient("any").Return(chRecv, cleanup)

	uc := clientsuc.NewUseCase(allowAllClientsEnforcer{}, active, hist)

	gotCh, gotCleanup, err := uc.SubscribeClient(clientsTestCtx(), "any")
	require.NoError(t, err)
	require.NotNil(t, gotCh)
	gotCleanup()
}

func TestUseCase_SubscribeChanges_DelegatesToActive(t *testing.T) {
	t.Parallel()

	ch := make(chan domain.ClientChange)

	ctrl := gomock.NewController(t)
	active := clients_mock.NewMockActiveSource(ctrl)
	hist := clients_mock.NewMockHistorySource(ctrl)

	var chRecv <-chan domain.ClientChange = ch
	active.EXPECT().Subscribe().Return(chRecv, func() { close(ch) })

	uc := clientsuc.NewUseCase(allowAllClientsEnforcer{}, active, hist)

	gotCh, cleanup, err := uc.SubscribeChanges(clientsTestCtx())
	require.NoError(t, err)
	require.NotNil(t, gotCh)
	cleanup()

	_, ok := <-gotCh
	assert.False(t, ok, "cleanup must close the channel")
}

package dashboard

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/sergeyslonimsky/elara/internal/domain"
	v2 "github.com/sergeyslonimsky/elara/internal/handler/v2"
	configv1 "github.com/sergeyslonimsky/elara/internal/proto/elara/config/v1"
	dashboardv1 "github.com/sergeyslonimsky/elara/internal/proto/elara/dashboard/v1"
	dashboarduc "github.com/sergeyslonimsky/elara/internal/usecase/dashboard"
)

//go:generate mockgen -destination=mocks/handler_mock.go -package=dashboard_mock -source=handler.go

type usecase interface {
	GetStats(ctx context.Context) (*dashboarduc.StatsResult, error)
	ListActivity(ctx context.Context, limit int) ([]*domain.ChangelogEntry, error)
}

// Handler implements dashboardv1connect.DashboardServiceHandler.
//
// GetStats and ListActivity are gate-less at the handler boundary; the use case
// filters results per-namespace using the PDP so each caller sees only the
// namespaces they can read.
type Handler struct {
	uc usecase
}

func New(uc usecase) *Handler {
	return &Handler{uc: uc}
}

func (h *Handler) GetStats(
	ctx context.Context,
	_ *connect.Request[dashboardv1.GetStatsRequest],
) (*connect.Response[dashboardv1.GetStatsResponse], error) {
	stats, err := h.uc.GetStats(ctx)
	if err != nil {
		return nil, v2.ToConnectError(err)
	}

	return connect.NewResponse(&dashboardv1.GetStatsResponse{
		NamespaceCount:    int32(stats.NamespaceCount),
		ConfigCount:       int32(stats.ConfigCount),
		ActiveClientCount: int32(stats.ActiveClientCount),
		GlobalRevision:    stats.GlobalRevision,
	}), nil
}

func (h *Handler) ListActivity(
	ctx context.Context,
	req *connect.Request[dashboardv1.ListActivityRequest],
) (*connect.Response[dashboardv1.ListActivityResponse], error) {
	limit, err := v2.NormalizeLimit(req.Msg.GetLimit())
	if err != nil {
		return nil, fmt.Errorf("normalize limit: %w", err)
	}

	entries, err := h.uc.ListActivity(ctx, limit)
	if err != nil {
		return nil, v2.ToConnectError(err)
	}

	proto := make([]*dashboardv1.ActivityEntry, 0, len(entries))
	for _, e := range entries {
		proto = append(proto, changelogEntryToActivityProto(e))
	}

	return connect.NewResponse(&dashboardv1.ListActivityResponse{
		Entries: proto,
	}), nil
}

func changelogEntryToActivityProto(e *domain.ChangelogEntry) *dashboardv1.ActivityEntry {
	entry := &dashboardv1.ActivityEntry{
		Revision:  e.Revision,
		Path:      e.Path,
		Namespace: e.Namespace,
		Version:   e.Version,
		Timestamp: timestamppb.New(e.Timestamp),
	}

	switch e.Type {
	case domain.EventTypeCreated:
		entry.EventType = configv1.EventType_EVENT_TYPE_CREATED
	case domain.EventTypeUpdated:
		entry.EventType = configv1.EventType_EVENT_TYPE_UPDATED
	case domain.EventTypeDeleted:
		entry.EventType = configv1.EventType_EVENT_TYPE_DELETED
	case domain.EventTypeLocked:
		entry.EventType = configv1.EventType_EVENT_TYPE_LOCKED
	case domain.EventTypeUnlocked:
		entry.EventType = configv1.EventType_EVENT_TYPE_UNLOCKED
	case domain.EventTypeNamespaceLocked:
		entry.EventType = configv1.EventType_EVENT_TYPE_NAMESPACE_LOCKED
	case domain.EventTypeNamespaceUnlocked:
		entry.EventType = configv1.EventType_EVENT_TYPE_NAMESPACE_UNLOCKED
	}

	return entry
}

package v2

import (
	"context"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/sergeyslonimsky/elara/internal/domain"
	configv1 "github.com/sergeyslonimsky/elara/internal/proto/elara/config/v1"
	dashboardv1 "github.com/sergeyslonimsky/elara/internal/proto/elara/dashboard/v1"
	dashboarduc "github.com/sergeyslonimsky/elara/internal/usecase/dashboard"
)

// DashboardHandler implements dashboardv1connect.DashboardServiceHandler.
type DashboardHandler struct {
	uc *dashboarduc.UseCase
}

func NewDashboardHandler(uc *dashboarduc.UseCase) *DashboardHandler {
	return &DashboardHandler{uc: uc}
}

func (h *DashboardHandler) GetStats(
	ctx context.Context,
	_ *connect.Request[dashboardv1.GetStatsRequest],
) (*connect.Response[dashboardv1.GetStatsResponse], error) {
	stats, err := h.uc.GetStats(ctx)
	if err != nil {
		return nil, toConnectError(err)
	}

	return connect.NewResponse(&dashboardv1.GetStatsResponse{
		NamespaceCount:    int32(stats.NamespaceCount),
		ConfigCount:       int32(stats.ConfigCount),
		ActiveClientCount: int32(stats.ActiveClientCount),
		GlobalRevision:    stats.GlobalRevision,
	}), nil
}

func (h *DashboardHandler) ListActivity(
	ctx context.Context,
	req *connect.Request[dashboardv1.ListActivityRequest],
) (*connect.Response[dashboardv1.ListActivityResponse], error) {
	limit, err := normalizeLimit(req.Msg.GetLimit())
	if err != nil {
		return nil, err
	}

	entries, err := h.uc.ListActivity(ctx, limit)
	if err != nil {
		return nil, toConnectError(err)
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
	}

	return entry
}

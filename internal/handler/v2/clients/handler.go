package clients

import (
	"context"
	"errors"
	"fmt"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/sergeyslonimsky/elara/internal/domain"
	v2 "github.com/sergeyslonimsky/elara/internal/handler/v2"
	clientsv2 "github.com/sergeyslonimsky/elara/internal/proto/elara/clients/v1"
)

//go:generate mockgen -destination=mocks/handler_mock.go -package=clients_mock -source=handler.go

// defaultWatchSnapshotInterval is how often WatchClients pushes a periodic
// snapshot. Connect/Disconnect events are pushed immediately on top of this.
const defaultWatchSnapshotInterval = 2 * time.Second

var errClientNotFound = errors.New("client not found")

type (
	authz interface {
		Require(ctx context.Context, object, action, domainStr string) error
	}

	usecase interface {
		ListActive(ctx context.Context) ([]*domain.Client, error)
		ListHistorical(ctx context.Context, limit int) ([]*domain.Client, error)
		ListSessions(
			ctx context.Context,
			clientName, k8sNamespace, currentID string,
			limit int,
		) ([]*domain.Client, error)
		Get(ctx context.Context, id string) (*domain.Client, []domain.ClientEvent, error)
		SubscribeChanges(ctx context.Context) (<-chan domain.ClientChange, func(), error)
		SubscribeClient(ctx context.Context, connID string) (<-chan domain.ClientChange, func(), error)
	}
)

type Handler struct {
	authz            authz
	uc               usecase
	snapshotInterval time.Duration
}

// Option configures a Handler. Pass to NewHandler.
type Option func(*Handler)

// WithSnapshotInterval sets how often WatchClients/WatchClient emit a
// SNAPSHOT frame. Zero or negative falls back to defaultWatchSnapshotInterval.
// Production use: tune heartbeat for chatty/quiet streams. Tests: shorten to
// drive ticker assertions, or set very long to suppress ticker entirely.
func WithSnapshotInterval(d time.Duration) Option {
	return func(h *Handler) { h.snapshotInterval = d }
}

func NewHandler(authz authz, uc usecase, opts ...Option) *Handler {
	h := &Handler{authz: authz, uc: uc}
	for _, opt := range opts {
		opt(h)
	}

	return h
}

func (h *Handler) ListActiveClients(
	ctx context.Context,
	_ *connect.Request[clientsv2.ListActiveClientsRequest],
) (*connect.Response[clientsv2.ListActiveClientsResponse], error) {
	clients, err := h.uc.ListActive(ctx)
	if err != nil {
		return nil, fmt.Errorf("list active clients: %w", err)
	}

	resp := &clientsv2.ListActiveClientsResponse{
		Clients: make([]*clientsv2.Client, 0, len(clients)),
	}

	for _, c := range clients {
		resp.Clients = append(resp.Clients, domainClientToProto(c))
	}

	return connect.NewResponse(resp), nil
}

func (h *Handler) GetClient(
	ctx context.Context,
	req *connect.Request[clientsv2.GetClientRequest],
) (*connect.Response[clientsv2.GetClientResponse], error) {
	if err := h.authz.Require(ctx, domain.ObjectClient, domain.ActionRead, domain.DomainAll); err != nil {
		return nil, v2.ToConnectError(err)
	}

	client, recent, err := h.uc.Get(ctx, req.Msg.GetId())
	if err != nil {
		return nil, v2.ToConnectError(err)
	}

	if client == nil {
		return nil, connect.NewError(connect.CodeNotFound, errClientNotFound)
	}

	resp := &clientsv2.GetClientResponse{
		Client:       domainClientToProto(client),
		RecentEvents: make([]*clientsv2.ClientEvent, 0, len(recent)),
	}

	for _, e := range recent {
		resp.RecentEvents = append(resp.RecentEvents, domainClientEventToProto(e))
	}

	return connect.NewResponse(resp), nil
}

func (h *Handler) ListHistoricalConnections(
	ctx context.Context,
	req *connect.Request[clientsv2.ListHistoricalConnectionsRequest],
) (*connect.Response[clientsv2.ListHistoricalConnectionsResponse], error) {
	limit, err := v2.NormalizeLimit(req.Msg.GetLimit())
	if err != nil {
		return nil, fmt.Errorf("normalize limit: %w", err)
	}

	hist, err := h.uc.ListHistorical(ctx, limit)
	if err != nil {
		return nil, v2.ToConnectError(err)
	}

	resp := &clientsv2.ListHistoricalConnectionsResponse{
		Clients: make([]*clientsv2.Client, 0, len(hist)),
	}

	for _, c := range hist {
		resp.Clients = append(resp.Clients, domainClientToProto(c))
	}

	return connect.NewResponse(resp), nil
}

func (h *Handler) ListClientSessions(
	ctx context.Context,
	req *connect.Request[clientsv2.ListClientSessionsRequest],
) (*connect.Response[clientsv2.ListClientSessionsResponse], error) {
	limit, err := v2.NormalizeLimit(req.Msg.GetLimit())
	if err != nil {
		return nil, fmt.Errorf("normalize limit: %w", err)
	}

	sessions, err := h.uc.ListSessions(
		ctx,
		req.Msg.GetClientName(),
		req.Msg.GetK8SNamespace(),
		req.Msg.GetCurrentId(),
		limit,
	)
	if err != nil {
		return nil, v2.ToConnectError(err)
	}

	resp := &clientsv2.ListClientSessionsResponse{
		Sessions: make([]*clientsv2.Client, 0, len(sessions)),
	}

	for _, s := range sessions {
		resp.Sessions = append(resp.Sessions, domainClientToProto(s))
	}

	return connect.NewResponse(resp), nil
}

// watchSender abstracts the bit of connect.ServerStream the watch loop needs.
// Implemented by *connect.ServerStream and by test fakes.
type watchSender interface {
	Send(resp *clientsv2.WatchClientsResponse) error
}

// watchClientSender is the single-client equivalent.
type watchClientSender interface {
	Send(resp *clientsv2.WatchClientResponse) error
}

// WatchClients is a server-streaming RPC: initial snapshot + periodic snapshots
// + immediate single-client events on Connected/Disconnected.
//
// Memory-leak safety:
//   - subscription to UseCase.SubscribeChanges is unsubscribed in defer
//   - ticker is stopped in defer
//   - exits cleanly on stream context cancel or send error
func (h *Handler) WatchClients(
	ctx context.Context,
	_ *connect.Request[clientsv2.WatchClientsRequest],
	stream *connect.ServerStream[clientsv2.WatchClientsResponse],
) error {
	if err := h.authz.Require(ctx, domain.ObjectClient, domain.ActionRead, domain.DomainAll); err != nil {
		return v2.ToConnectError(err)
	}

	return h.runWatch(ctx, stream)
}

// WatchClient streams updates for a single client. Sends an immediate snapshot,
// then forwards per-RPC events plus a periodic snapshot (so KPI counters stay
// fresh even between RPCs). Stream closes when the client disconnects.
//
// Memory-leak safety:
//   - SubscribeClient is unsubscribed in defer
//   - ticker is stopped in defer
//   - exits cleanly on stream context cancel, send error, or disconnect
func (h *Handler) WatchClient(
	ctx context.Context,
	req *connect.Request[clientsv2.WatchClientRequest],
	stream *connect.ServerStream[clientsv2.WatchClientResponse],
) error {
	if err := h.authz.Require(ctx, domain.ObjectClient, domain.ActionRead, domain.DomainAll); err != nil {
		return v2.ToConnectError(err)
	}

	return h.runWatchClient(ctx, req.Msg.GetId(), stream)
}

// runWatch is the inner loop, exposed against an interface for unit testing.
func (h *Handler) runWatch(ctx context.Context, sender watchSender) error {
	interval := h.snapshotInterval
	if interval <= 0 {
		interval = defaultWatchSnapshotInterval
	}

	changes, cancel, err := h.uc.SubscribeChanges(ctx)
	if err != nil {
		return v2.ToConnectError(err)
	}

	defer cancel()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Initial snapshot so the UI has data without waiting for the first tick.
	if err := h.sendSnapshot(ctx, sender); err != nil {
		return err
	}

	return h.watchLoop(ctx, sender, changes, ticker)
}

func (h *Handler) watchLoop(
	ctx context.Context,
	sender watchSender,
	changes <-chan domain.ClientChange,
	ticker *time.Ticker,
) error {
	for {
		select {
		case <-ctx.Done():
			return nil // client closed the stream

		case <-ticker.C:
			if err := h.sendSnapshot(ctx, sender); err != nil {
				return err
			}

		case ev, ok := <-changes:
			if !ok {
				return nil // registry shut down
			}

			if err := h.sendChange(sender, ev); err != nil {
				return err
			}
		}
	}
}

func (h *Handler) sendSnapshot(ctx context.Context, sender watchSender) error {
	clients, err := h.uc.ListActive(ctx)
	if err != nil {
		return fmt.Errorf("list active clients: %w", err)
	}

	protoClients := make([]*clientsv2.Client, 0, len(clients))

	for _, c := range clients {
		protoClients = append(protoClients, domainClientToProto(c))
	}

	if err := sender.Send(&clientsv2.WatchClientsResponse{
		Kind:    clientsv2.WatchClientsResponse_KIND_SNAPSHOT,
		Clients: protoClients,
	}); err != nil {
		return fmt.Errorf("send clients snapshot: %w", err)
	}

	return nil
}

func (h *Handler) runWatchClient(ctx context.Context, id string, sender watchClientSender) error {
	ok, err := h.initWatchClient(ctx, id, sender)
	if err != nil || !ok {
		return err
	}

	changes, cancel, err := h.uc.SubscribeClient(ctx, id)
	if err != nil {
		return v2.ToConnectError(err)
	}

	defer cancel()

	interval := h.snapshotInterval
	if interval <= 0 {
		interval = defaultWatchSnapshotInterval
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	return h.watchClientLoop(ctx, id, sender, changes, ticker)
}

// initWatchClient resolves the client, sends the initial frame, and returns
// whether the caller should proceed with the event loop. Returns (false, nil)
// when the stream has already been completed (e.g. client disconnected).
func (h *Handler) initWatchClient(
	ctx context.Context,
	id string,
	sender watchClientSender,
) (bool, error) {
	c, _, err := h.uc.Get(ctx, id)
	if err != nil {
		return false, v2.ToConnectError(err)
	}

	if c == nil {
		return false, connect.NewError(connect.CodeNotFound, errClientNotFound)
	}

	if !c.IsActive() {
		if err := sender.Send(&clientsv2.WatchClientResponse{
			Kind:   clientsv2.WatchClientResponse_KIND_DISCONNECTED,
			Client: domainClientToProto(c),
		}); err != nil {
			return false, fmt.Errorf("send disconnected client: %w", err)
		}

		return false, nil
	}

	if err := sender.Send(&clientsv2.WatchClientResponse{
		Kind:   clientsv2.WatchClientResponse_KIND_SNAPSHOT,
		Client: domainClientToProto(c),
	}); err != nil {
		return false, fmt.Errorf("send client snapshot: %w", err)
	}

	return true, nil
}

// sendTickerSnapshot fetches the latest client state and sends a SNAPSHOT frame.
// Returns (true, nil) when the client has disconnected and the loop should exit.
func (h *Handler) sendTickerSnapshot(
	ctx context.Context,
	id string,
	sender watchClientSender,
) (bool, error) {
	snap, _, err := h.uc.Get(ctx, id)
	if err != nil {
		return false, v2.ToConnectError(err)
	}

	if snap == nil || !snap.IsActive() {
		return true, nil
	}

	if err := sender.Send(&clientsv2.WatchClientResponse{
		Kind:   clientsv2.WatchClientResponse_KIND_SNAPSHOT,
		Client: domainClientToProto(snap),
	}); err != nil {
		return false, fmt.Errorf("send ticker snapshot: %w", err)
	}

	return false, nil
}

func (h *Handler) watchClientLoop(
	ctx context.Context,
	id string,
	sender watchClientSender,
	changes <-chan domain.ClientChange,
	ticker *time.Ticker,
) error {
	for {
		select {
		case <-ctx.Done():
			return nil

		case <-ticker.C:
			done, err := h.sendTickerSnapshot(ctx, id, sender)
			if err != nil {
				return err
			}

			if done {
				return nil
			}

		case ev, ok := <-changes:
			if !ok {
				return nil
			}

			if err := h.sendClientChange(sender, ev); err != nil {
				return err
			}

			if ev.Kind == domain.ClientDisconnected {
				return nil
			}
		}
	}
}

func (h *Handler) sendClientChange(sender watchClientSender, ev domain.ClientChange) error {
	switch ev.Kind {
	case domain.ClientRequestRecorded:
		if ev.Event == nil {
			return nil
		}

		if err := sender.Send(&clientsv2.WatchClientResponse{
			Kind:   clientsv2.WatchClientResponse_KIND_REQUEST_RECORDED,
			Client: domainClientToProto(ev.Client),
			Event:  domainClientEventToProto(*ev.Event),
		}); err != nil {
			return fmt.Errorf("send request recorded event: %w", err)
		}

		return nil

	case domain.ClientDisconnected:
		if err := sender.Send(&clientsv2.WatchClientResponse{
			Kind:   clientsv2.WatchClientResponse_KIND_DISCONNECTED,
			Client: domainClientToProto(ev.Client),
		}); err != nil {
			return fmt.Errorf("send disconnected event: %w", err)
		}

		return nil

	default:
		// Activity / Connected events are handled by ticker snapshots — skip.
		return nil
	}
}

func (h *Handler) sendChange(sender watchSender, ev domain.ClientChange) error {
	var kind clientsv2.WatchClientsResponse_Kind
	switch ev.Kind {
	case domain.ClientConnected:
		kind = clientsv2.WatchClientsResponse_KIND_CONNECTED
	case domain.ClientDisconnected:
		kind = clientsv2.WatchClientsResponse_KIND_DISCONNECTED
	default:
		// ClientActivity events are folded into the next snapshot tick — they
		// don't carry enough new info to justify their own message.
		return nil
	}

	if err := sender.Send(&clientsv2.WatchClientsResponse{
		Kind:    kind,
		Clients: []*clientsv2.Client{domainClientToProto(ev.Client)},
	}); err != nil {
		return fmt.Errorf("send client change: %w", err)
	}

	return nil
}

// -----------------------------------------------------------------------------
// Converters
// -----------------------------------------------------------------------------

func domainClientToProto(c *domain.Client) *clientsv2.Client {
	if c == nil {
		return nil
	}

	out := &clientsv2.Client{
		Id:             c.ID,
		PeerAddress:    c.PeerAddress,
		UserAgent:      c.UserAgent,
		ClientName:     c.ClientName,
		ClientVersion:  c.ClientVersion,
		K8SNamespace:   c.K8sNamespace,
		K8SPod:         c.K8sPod,
		K8SNode:        c.K8sNode,
		InstanceId:     c.InstanceID,
		ConnectedAt:    timestamppb.New(c.ConnectedAt),
		LastActivityAt: timestamppb.New(c.LastActivityAt),
		ActiveWatches:  c.ActiveWatches,
		RequestCounts:  c.RequestCounts,
		ErrorCount:     c.ErrorCount,
	}

	if c.DisconnectedAt != nil {
		out.DisconnectedAt = timestamppb.New(*c.DisconnectedAt)
	}

	if len(c.ActiveWatchList) > 0 {
		out.ActiveWatchList = make([]*clientsv2.ActiveWatch, 0, len(c.ActiveWatchList))
		for _, w := range c.ActiveWatchList {
			out.ActiveWatchList = append(out.ActiveWatchList, &clientsv2.ActiveWatch{
				WatchId:        w.WatchID,
				StartKey:       w.StartKey,
				EndKey:         w.EndKey,
				StartRevision:  w.StartRevision,
				CreatedAt:      timestamppb.New(w.CreatedAt),
				PrevKv:         w.PrevKv,
				ProgressNotify: w.ProgressNotify,
			})
		}
	}

	return out
}

func domainClientEventToProto(e domain.ClientEvent) *clientsv2.ClientEvent {
	return &clientsv2.ClientEvent{
		Timestamp: timestamppb.New(e.Timestamp),
		Method:    e.Method,
		Key:       e.Key,
		Revision:  e.Revision,
		Duration:  durationpb.New(e.Duration),
		Error:     e.Error,
	}
}

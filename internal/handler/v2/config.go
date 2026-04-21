package v2

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/sergeyslonimsky/elara/internal/domain"
	commonv1 "github.com/sergeyslonimsky/elara/internal/proto/elara/common/v1"
	configv2 "github.com/sergeyslonimsky/elara/internal/proto/elara/config/v1"
	configuc "github.com/sergeyslonimsky/elara/internal/usecase/config"
)

type ConfigHandler struct {
	create   *configuc.CreateUseCase
	get      *configuc.GetUseCase
	update   *configuc.UpdateUseCase
	del      *configuc.DeleteUseCase
	list     *configuc.ListUseCase
	history  *configuc.HistoryUseCase
	search   *configuc.SearchUseCase
	copy     *configuc.CopyUseCase
	validate *configuc.ValidateUseCase
	watch    *configuc.WatchUseCase
	diff     *configuc.DiffUseCase
	lock     *configuc.LockUseCase
	unlock   *configuc.UnlockUseCase
}

func NewConfigHandler(
	create *configuc.CreateUseCase,
	get *configuc.GetUseCase,
	update *configuc.UpdateUseCase,
	del *configuc.DeleteUseCase,
	list *configuc.ListUseCase,
	history *configuc.HistoryUseCase,
	search *configuc.SearchUseCase,
	copyCfg *configuc.CopyUseCase,
	validate *configuc.ValidateUseCase,
	watch *configuc.WatchUseCase,
	diff *configuc.DiffUseCase,
	lock *configuc.LockUseCase,
	unlock *configuc.UnlockUseCase,
) *ConfigHandler {
	return &ConfigHandler{
		create:   create,
		get:      get,
		update:   update,
		del:      del,
		list:     list,
		history:  history,
		search:   search,
		copy:     copyCfg,
		validate: validate,
		watch:    watch,
		diff:     diff,
		lock:     lock,
		unlock:   unlock,
	}
}

func (h *ConfigHandler) CreateConfig(
	ctx context.Context,
	req *connect.Request[configv2.CreateConfigRequest],
) (*connect.Response[configv2.CreateConfigResponse], error) {
	cfg := &domain.Config{
		Path:      req.Msg.GetPath(),
		Content:   req.Msg.GetContent(),
		Format:    protoFormatToDomain(req.Msg.GetFormat()),
		Namespace: req.Msg.GetNamespace(),
		Metadata:  req.Msg.GetMetadata(),
	}

	result, err := h.create.Execute(ctx, cfg)
	if err != nil {
		return nil, toConnectError(err)
	}

	return connect.NewResponse(&configv2.CreateConfigResponse{
		Config: domainConfigToProto(result),
	}), nil
}

func (h *ConfigHandler) GetConfig(
	ctx context.Context,
	req *connect.Request[configv2.GetConfigRequest],
) (*connect.Response[configv2.GetConfigResponse], error) {
	result, err := h.get.Execute(ctx, req.Msg.GetPath(), req.Msg.GetNamespace())
	if err != nil {
		return nil, toConnectError(err)
	}

	return connect.NewResponse(&configv2.GetConfigResponse{
		Config: domainConfigToProto(result),
	}), nil
}

func (h *ConfigHandler) UpdateConfig(
	ctx context.Context,
	req *connect.Request[configv2.UpdateConfigRequest],
) (*connect.Response[configv2.UpdateConfigResponse], error) {
	cfg := &domain.Config{
		Path:      req.Msg.GetPath(),
		Content:   req.Msg.GetContent(),
		Format:    protoFormatToDomain(req.Msg.GetFormat()),
		Namespace: req.Msg.GetNamespace(),
		Version:   req.Msg.GetVersion(),
		Metadata:  req.Msg.GetMetadata(),
	}

	result, err := h.update.Execute(ctx, cfg)
	if err != nil {
		return nil, toConnectError(err)
	}

	return connect.NewResponse(&configv2.UpdateConfigResponse{
		Config: domainConfigToProto(result),
	}), nil
}

func (h *ConfigHandler) DeleteConfig(
	ctx context.Context,
	req *connect.Request[configv2.DeleteConfigRequest],
) (*connect.Response[configv2.DeleteConfigResponse], error) {
	if err := h.del.Execute(ctx, req.Msg.GetPath(), req.Msg.GetNamespace()); err != nil {
		return nil, toConnectError(err)
	}

	return connect.NewResponse(&configv2.DeleteConfigResponse{}), nil
}

func (h *ConfigHandler) ListConfigs(
	ctx context.Context,
	req *connect.Request[configv2.ListConfigsRequest],
) (*connect.Response[configv2.ListConfigsResponse], error) {
	params := configuc.ListParams{
		Namespace: req.Msg.GetNamespace(),
		Path:      req.Msg.GetPath(),
		Sort:      protoSortToDomain(req.Msg.GetSort()),
		Query:     req.Msg.GetQuery(),
	}

	if p := req.Msg.GetPagination(); p != nil {
		limit, err := normalizeLimit(p.GetLimit())
		if err != nil {
			return nil, err
		}

		offset, err := normalizeOffset(p.GetOffset())
		if err != nil {
			return nil, err
		}

		params.Limit = limit
		params.Offset = offset
	}

	result, err := h.list.Execute(ctx, params)
	if err != nil {
		return nil, toConnectError(err)
	}

	entries := make([]*configv2.DirectoryEntry, 0, len(result.Entries))
	for _, e := range result.Entries {
		entries = append(entries, directoryEntryToProto(e))
	}

	return connect.NewResponse(&configv2.ListConfigsResponse{
		Entries: entries,
		Pagination: &commonv1.PaginationResponse{
			Total:  int32(result.Total),
			Limit:  int32(result.Limit),
			Offset: int32(result.Offset),
		},
	}), nil
}

func (h *ConfigHandler) GetConfigHistory(
	ctx context.Context,
	req *connect.Request[configv2.GetConfigHistoryRequest],
) (*connect.Response[configv2.GetConfigHistoryResponse], error) {
	limit, err := normalizeLimit(req.Msg.GetLimit())
	if err != nil {
		return nil, err
	}

	entries, err := h.history.GetHistory(ctx, req.Msg.GetPath(), req.Msg.GetNamespace(), limit)
	if err != nil {
		return nil, toConnectError(err)
	}

	protos := make([]*configv2.HistoryEntry, 0, len(entries))
	for _, e := range entries {
		protos = append(protos, domainHistoryEntryToProto(e))
	}

	return connect.NewResponse(&configv2.GetConfigHistoryResponse{
		Entries: protos,
	}), nil
}

func (h *ConfigHandler) GetConfigAtRevision(
	ctx context.Context,
	req *connect.Request[configv2.GetConfigAtRevisionRequest],
) (*connect.Response[configv2.GetConfigAtRevisionResponse], error) {
	entry, err := h.history.GetAtRevision(ctx, req.Msg.GetPath(), req.Msg.GetNamespace(), req.Msg.GetRevision())
	if err != nil {
		return nil, toConnectError(err)
	}

	return connect.NewResponse(&configv2.GetConfigAtRevisionResponse{
		Entry: domainHistoryEntryToProto(entry),
	}), nil
}

func (h *ConfigHandler) SearchConfigs(
	ctx context.Context,
	req *connect.Request[configv2.SearchConfigsRequest],
) (*connect.Response[configv2.SearchConfigsResponse], error) {
	params := configuc.SearchParams{
		Query:     req.Msg.GetQuery(),
		Namespace: req.Msg.GetNamespace(),
		Sort:      protoSortToDomain(req.Msg.GetSort()),
	}

	if p := req.Msg.GetPagination(); p != nil {
		limit, err := normalizeLimit(p.GetLimit())
		if err != nil {
			return nil, err
		}

		offset, err := normalizeOffset(p.GetOffset())
		if err != nil {
			return nil, err
		}

		params.Limit = limit
		params.Offset = offset
	}

	result, err := h.search.Execute(ctx, params)
	if err != nil {
		return nil, toConnectError(err)
	}

	protos := make([]*configv2.ConfigSummary, 0, len(result.Results))
	for _, s := range result.Results {
		protos = append(protos, domainSummaryToProto(s))
	}

	return connect.NewResponse(&configv2.SearchConfigsResponse{
		Results: protos,
		Pagination: &commonv1.PaginationResponse{
			Total:  int32(result.Total),
			Limit:  int32(result.Limit),
			Offset: int32(result.Offset),
		},
	}), nil
}

func (h *ConfigHandler) CopyConfig(
	ctx context.Context,
	req *connect.Request[configv2.CopyConfigRequest],
) (*connect.Response[configv2.CopyConfigResponse], error) {
	result, err := h.copy.Execute(
		ctx,
		req.Msg.GetSourcePath(), req.Msg.GetSourceNamespace(),
		req.Msg.GetDestinationPath(), req.Msg.GetDestinationNamespace(),
	)
	if err != nil {
		return nil, toConnectError(err)
	}

	return connect.NewResponse(&configv2.CopyConfigResponse{
		Config: domainConfigToProto(result),
	}), nil
}

func (h *ConfigHandler) ValidateConfig(
	ctx context.Context,
	req *connect.Request[configv2.ValidateConfigRequest],
) (*connect.Response[configv2.ValidateConfigResponse], error) {
	result, err := h.validate.Execute(req.Msg.GetContent(), protoFormatToDomain(req.Msg.GetFormat()))
	if err != nil {
		return nil, toConnectError(err)
	}

	return connect.NewResponse(&configv2.ValidateConfigResponse{
		Result: &configv2.ValidationResult{
			Valid:             result.Valid,
			Errors:            result.Errors,
			DetectedFormat:    domainFormatToProto(result.DetectedFormat),
			NormalizedContent: result.NormalizedContent,
		},
	}), nil
}

func (h *ConfigHandler) GetConfigDiff(
	ctx context.Context,
	req *connect.Request[configv2.GetConfigDiffRequest],
) (*connect.Response[configv2.GetConfigDiffResponse], error) {
	result, err := h.diff.GetDiff(ctx,
		req.Msg.GetPath(),
		req.Msg.GetNamespace(),
		req.Msg.GetFromRevision(),
		req.Msg.GetToRevision(),
	)
	if err != nil {
		return nil, toConnectError(err)
	}

	return connect.NewResponse(&configv2.GetConfigDiffResponse{
		FromRevision: result.FromRevision,
		ToRevision:   result.ToRevision,
		FromContent:  result.FromContent,
		ToContent:    result.ToContent,
		Diff:         result.Diff,
	}), nil
}

func (h *ConfigHandler) WatchConfigs(
	ctx context.Context,
	req *connect.Request[configv2.WatchConfigsRequest],
	stream *connect.ServerStream[configv2.WatchConfigsResponse],
) error {
	events, cancel := h.watch.Execute(ctx, req.Msg.GetPathPrefix(), req.Msg.GetNamespace())
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("watch configs: %w", ctx.Err())
		case event, ok := <-events:
			if !ok {
				return nil
			}

			resp := &configv2.WatchConfigsResponse{
				Event: domainWatchEventToProto(&event),
			}

			if err := stream.Send(resp); err != nil {
				return fmt.Errorf("send watch event: %w", err)
			}
		}
	}
}

func (h *ConfigHandler) LockConfig(
	ctx context.Context,
	req *connect.Request[configv2.LockConfigRequest],
) (*connect.Response[configv2.LockConfigResponse], error) {
	if err := h.lock.Execute(ctx, req.Msg.GetNamespace(), req.Msg.GetPath()); err != nil {
		return nil, toConnectError(err)
	}

	return connect.NewResponse(&configv2.LockConfigResponse{}), nil
}

func (h *ConfigHandler) UnlockConfig(
	ctx context.Context,
	req *connect.Request[configv2.UnlockConfigRequest],
) (*connect.Response[configv2.UnlockConfigResponse], error) {
	if err := h.unlock.Execute(ctx, req.Msg.GetNamespace(), req.Msg.GetPath()); err != nil {
		return nil, toConnectError(err)
	}

	return connect.NewResponse(&configv2.UnlockConfigResponse{}), nil
}

// Converters

func domainConfigToProto(cfg *domain.Config) *configv2.Config {
	if cfg == nil {
		return nil
	}

	return &configv2.Config{
		Path:        cfg.Path,
		Content:     cfg.Content,
		ContentHash: cfg.ContentHash,
		Format:      domainFormatToProto(cfg.Format),
		Namespace:   cfg.Namespace,
		Version:     cfg.Version,
		Revision:    cfg.Revision,
		Metadata:    cfg.Metadata,
		Locked:      cfg.Locked,
		CreatedAt:   timestamppb.New(cfg.CreatedAt),
		UpdatedAt:   timestamppb.New(cfg.UpdatedAt),
	}
}

func directoryEntryToProto(e *configuc.DirectoryEntry) *configv2.DirectoryEntry {
	entry := &configv2.DirectoryEntry{
		Name:     e.Name,
		FullPath: e.FullPath,
		IsFile:   e.IsFile,
	}

	if e.IsFile {
		entry.Format = domainFormatToProto(e.Format)
		entry.Version = e.Version
		entry.Revision = e.Revision
		entry.UpdatedAt = timestamppb.New(e.UpdatedAt)
		entry.Locked = e.Locked
	} else {
		entry.ChildCount = int32(e.ChildCount)
		entry.UpdatedAt = timestamppb.New(e.UpdatedAt)
	}

	return entry
}

func domainSummaryToProto(s *domain.ConfigSummary) *configv2.ConfigSummary {
	if s == nil {
		return nil
	}

	return &configv2.ConfigSummary{
		Path:        s.Path,
		ContentHash: s.ContentHash,
		Format:      domainFormatToProto(s.Format),
		Namespace:   s.Namespace,
		Version:     s.Version,
		Revision:    s.Revision,
		Metadata:    s.Metadata,
		Locked:      s.Locked,
		CreatedAt:   timestamppb.New(s.CreatedAt),
		UpdatedAt:   timestamppb.New(s.UpdatedAt),
	}
}

func domainHistoryEntryToProto(e *domain.HistoryEntry) *configv2.HistoryEntry {
	if e == nil {
		return nil
	}

	entry := &configv2.HistoryEntry{
		Revision:    e.Revision,
		Content:     e.Content,
		ContentHash: e.ContentHash,
		Timestamp:   timestamppb.New(e.Timestamp),
	}

	switch e.EventType {
	case domain.EventTypeCreated:
		entry.EventType = configv2.EventType_EVENT_TYPE_CREATED
	case domain.EventTypeUpdated:
		entry.EventType = configv2.EventType_EVENT_TYPE_UPDATED
	case domain.EventTypeDeleted:
		entry.EventType = configv2.EventType_EVENT_TYPE_DELETED
	case domain.EventTypeLocked:
		entry.EventType = configv2.EventType_EVENT_TYPE_LOCKED
	case domain.EventTypeUnlocked:
		entry.EventType = configv2.EventType_EVENT_TYPE_UNLOCKED
	}

	return entry
}

func domainFormatToProto(f domain.Format) configv2.Format {
	switch f {
	case domain.FormatJSON:
		return configv2.Format_FORMAT_JSON
	case domain.FormatYAML:
		return configv2.Format_FORMAT_YAML
	case domain.FormatOther:
		return configv2.Format_FORMAT_OTHER
	default:
		return configv2.Format_FORMAT_UNSPECIFIED
	}
}

func protoFormatToDomain(f configv2.Format) domain.Format {
	switch f {
	case configv2.Format_FORMAT_JSON:
		return domain.FormatJSON
	case configv2.Format_FORMAT_YAML:
		return domain.FormatYAML
	case configv2.Format_FORMAT_OTHER:
		return domain.FormatOther
	case configv2.Format_FORMAT_UNSPECIFIED:
		return ""
	default:
		return ""
	}
}

func protoSortToDomain(s *commonv1.SortRequest) domain.SortParams {
	if s == nil {
		return domain.SortParams{}
	}

	return domain.SortParams{
		Field: s.GetField(),
		Desc:  s.GetDirection() == commonv1.SortDirection_SORT_DIRECTION_DESC,
	}
}

func domainWatchEventToProto(event *domain.WatchEvent) *configv2.WatchEvent {
	if event == nil {
		return nil
	}

	we := &configv2.WatchEvent{
		Path:      event.Path,
		Namespace: event.Namespace,
		Config:    domainConfigToProto(event.Config),
		Timestamp: timestamppb.New(event.Timestamp),
	}

	switch event.Type {
	case domain.EventTypeCreated:
		we.Type = configv2.EventType_EVENT_TYPE_CREATED
	case domain.EventTypeUpdated:
		we.Type = configv2.EventType_EVENT_TYPE_UPDATED
	case domain.EventTypeDeleted:
		we.Type = configv2.EventType_EVENT_TYPE_DELETED
	case domain.EventTypeLocked:
		we.Type = configv2.EventType_EVENT_TYPE_LOCKED
	case domain.EventTypeUnlocked:
		we.Type = configv2.EventType_EVENT_TYPE_UNLOCKED
	}

	return we
}

package token

import (
	"context"
	"fmt"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/sergeyslonimsky/elara/internal/domain"
	v2 "github.com/sergeyslonimsky/elara/internal/handler/v2"
	commonv1 "github.com/sergeyslonimsky/elara/internal/proto/elara/common/v1"
	tokenv1 "github.com/sergeyslonimsky/elara/internal/proto/elara/token/v1"
	"github.com/sergeyslonimsky/elara/internal/service/auth"
	tokenuc "github.com/sergeyslonimsky/elara/internal/usecase/token"
)

//go:generate mockgen -destination=mocks/handler_mock.go -package=token_mock -source=handler.go

type (
	authz interface {
		Require(
			ctx context.Context,
			object domain.Object,
			action domain.Action,
			domainStr string,
		) error
	}

	usecase interface {
		Create(
			ctx context.Context,
			user domain.AuthInfo,
			in tokenuc.CreateInput,
		) (*domain.Token, string, error)
		List(
			ctx context.Context,
			user domain.AuthInfo,
			params tokenuc.ListParams,
		) (*tokenuc.ListResult, error)
		Get(ctx context.Context, user domain.AuthInfo, id string) (*domain.Token, error)
		Revoke(ctx context.Context, user domain.AuthInfo, id string) error
	}
)

// Handler implements tokenv1connect.TokenServiceHandler.
type Handler struct {
	authz authz
	uc    usecase
}

// New returns a new Handler.
func New(authz authz, uc usecase) *Handler {
	return &Handler{authz: authz, uc: uc}
}

func (h *Handler) CreateToken(
	ctx context.Context,
	req *connect.Request[tokenv1.CreateTokenRequest],
) (*connect.Response[tokenv1.CreateTokenResponse], error) {
	user, err := auth.AuthInfoFromContext(ctx)
	if err != nil {
		return nil, v2.ToConnectError(err)
	}

	for _, ns := range req.Msg.GetNamespaces() {
		if err := h.authz.Require(ctx, domain.ObjectToken, domain.ActionCreate, ns); err != nil {
			return nil, v2.ToConnectError(err)
		}
	}

	role, err := permissionActionToRole(req.Msg.GetPermission())
	if err != nil {
		return nil, v2.ToConnectError(err)
	}

	var expiresAt *time.Time
	if req.Msg.GetExpiresAt() != nil {
		expiresAt = new(req.Msg.GetExpiresAt().AsTime())
	}

	token, rawToken, err := h.uc.Create(ctx, user, tokenuc.CreateInput{
		Name:       req.Msg.GetName(),
		Namespaces: req.Msg.GetNamespaces(),
		Role:       role,
		ExpiresAt:  expiresAt,
	})
	if err != nil {
		return nil, v2.ToConnectError(err)
	}

	return connect.NewResponse(&tokenv1.CreateTokenResponse{
		Token:    domainTokenToProto(token),
		RawToken: rawToken,
	}), nil
}

func (h *Handler) ListTokens(
	ctx context.Context,
	req *connect.Request[tokenv1.ListTokensRequest],
) (*connect.Response[tokenv1.ListTokensResponse], error) {
	user, err := auth.AuthInfoFromContext(ctx)
	if err != nil {
		return nil, v2.ToConnectError(err)
	}

	params := tokenuc.ListParams{
		Sort: protoSortToDomain(req.Msg.GetSorting()),
	}

	if f := req.Msg.GetFilters(); f != nil {
		params.QueryParams = f.GetQueryParams()
		params.IssuedBy = f.GetIssuedBy()
		params.Namespaces = f.GetNamespaces()
	}

	if p := req.Msg.GetPagination(); p != nil {
		limit, err := v2.NormalizeLimit(p.GetLimit())
		if err != nil {
			return nil, fmt.Errorf("normalize limit: %w", err)
		}

		offset, err := v2.NormalizeOffset(p.GetOffset())
		if err != nil {
			return nil, fmt.Errorf("normalize offset: %w", err)
		}

		params.Limit = limit
		params.Offset = offset
	}

	result, err := h.uc.List(ctx, user, params)
	if err != nil {
		return nil, v2.ToConnectError(err)
	}

	protos := make([]*tokenv1.Token, 0, len(result.Tokens))
	for _, t := range result.Tokens {
		protos = append(protos, domainTokenToProto(t))
	}

	return connect.NewResponse(&tokenv1.ListTokensResponse{
		Tokens: protos,
		Pagination: &commonv1.PaginationResponse{
			Total:  int32(result.Total),
			Limit:  int32(result.Limit),
			Offset: int32(result.Offset),
		},
		Sorting: domainSortToProtoResponse(params.Sort),
	}), nil
}

func (h *Handler) GetToken(
	ctx context.Context,
	req *connect.Request[tokenv1.GetTokenRequest],
) (*connect.Response[tokenv1.GetTokenResponse], error) {
	user, err := auth.AuthInfoFromContext(ctx)
	if err != nil {
		return nil, v2.ToConnectError(err)
	}

	token, err := h.uc.Get(ctx, user, req.Msg.GetId())
	if err != nil {
		return nil, v2.ToConnectError(err)
	}

	return connect.NewResponse(&tokenv1.GetTokenResponse{Token: domainTokenToProto(token)}), nil
}

func (h *Handler) RevokeToken(
	ctx context.Context,
	req *connect.Request[tokenv1.RevokeTokenRequest],
) (*connect.Response[tokenv1.RevokeTokenResponse], error) {
	user, err := auth.AuthInfoFromContext(ctx)
	if err != nil {
		return nil, v2.ToConnectError(err)
	}

	if err := h.uc.Revoke(ctx, user, req.Msg.GetId()); err != nil {
		return nil, v2.ToConnectError(err)
	}

	return connect.NewResponse(&tokenv1.RevokeTokenResponse{}), nil
}

func domainTokenToProto(t *domain.Token) *tokenv1.Token {
	if t == nil {
		return nil
	}

	proto := &tokenv1.Token{
		Id:         t.ID,
		Name:       t.Name,
		IssuedBy:   t.IssuedBy,
		Namespaces: t.Namespaces,
		Permission: roleToPermissionAction(t.Role),
		LastUsedIp: t.LastUsedIP,
		CreatedAt:  timestamppb.New(t.CreatedAt),
	}

	if t.ExpiresAt != nil {
		proto.ExpiresAt = timestamppb.New(*t.ExpiresAt)
	}

	if t.LastUsedAt != nil {
		proto.LastUsedAt = timestamppb.New(*t.LastUsedAt)
	}

	return proto
}

// permissionActionToRole maps the proto-level permission enum to the domain
// role granted by a token. Tokens only support reader/writer roles, so any
// other value is rejected as a validation error.
func permissionActionToRole(a commonv1.PermissionAction) (domain.Role, error) {
	switch a {
	case commonv1.PermissionAction_PERMISSION_ACTION_READ:
		return domain.RoleReader, nil
	case commonv1.PermissionAction_PERMISSION_ACTION_WRITE:
		return domain.RoleWriter, nil
	case commonv1.PermissionAction_PERMISSION_ACTION_UNSPECIFIED,
		commonv1.PermissionAction_PERMISSION_ACTION_CREATE,
		commonv1.PermissionAction_PERMISSION_ACTION_ALL:
		return "", domain.NewValidationError("permission", "permission must be read or write")
	default:
		return "", domain.NewValidationError("permission", "unknown permission")
	}
}

func roleToPermissionAction(r domain.Role) commonv1.PermissionAction {
	switch r {
	case domain.RoleReader:
		return commonv1.PermissionAction_PERMISSION_ACTION_READ
	case domain.RoleWriter:
		return commonv1.PermissionAction_PERMISSION_ACTION_WRITE
	case domain.RoleAdmin:
		return commonv1.PermissionAction_PERMISSION_ACTION_ALL
	default:
		return commonv1.PermissionAction_PERMISSION_ACTION_UNSPECIFIED
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

func domainSortToProtoResponse(s domain.SortParams) *commonv1.SortResponse {
	if s.Field == "" {
		return nil
	}

	direction := commonv1.SortDirection_SORT_DIRECTION_ASC
	if s.Desc {
		direction = commonv1.SortDirection_SORT_DIRECTION_DESC
	}

	return &commonv1.SortResponse{
		Field:     s.Field,
		Direction: direction,
	}
}

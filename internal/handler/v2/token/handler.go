package token

import (
	"context"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/sergeyslonimsky/elara/internal/domain"
	v2 "github.com/sergeyslonimsky/elara/internal/handler/v2"
	tokenv1 "github.com/sergeyslonimsky/elara/internal/proto/elara/token/v1"
	tokenuc "github.com/sergeyslonimsky/elara/internal/usecase/token"
)

//go:generate mockgen -destination=mocks/handler_mock.go -package=token_mock -source=handler.go

type (
	authz interface {
		Require(ctx context.Context, object, action, domainStr string) error
	}

	usecase interface {
		Create(ctx context.Context, in tokenuc.CreateInput) (*domain.Token, string, error)
		List(ctx context.Context, params tokenuc.ListParams) (*tokenuc.ListResult, error)
		Get(ctx context.Context, id string) (*domain.Token, error)
		Revoke(ctx context.Context, id string) error
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
	for _, ns := range req.Msg.GetNamespaces() {
		if err := h.authz.Require(ctx, domain.ObjectToken, domain.ActionCreate, ns); err != nil {
			return nil, v2.ToConnectError(err)
		}
	}

	var expiresAt *time.Time
	if req.Msg.GetExpiresAt() != nil {
		t := req.Msg.GetExpiresAt().AsTime()
		expiresAt = &t
	}

	token, rawToken, err := h.uc.Create(ctx, tokenuc.CreateInput{
		Name:       req.Msg.GetName(),
		Namespaces: req.Msg.GetNamespaces(),
		Role:       req.Msg.GetRole(),
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
	result, err := h.uc.List(ctx, tokenuc.ListParams{
		IssuedBy: req.Msg.GetIssuedBy(),
	})
	if err != nil {
		return nil, v2.ToConnectError(err)
	}

	protos := make([]*tokenv1.Token, 0, len(result.Tokens))
	for _, t := range result.Tokens {
		protos = append(protos, domainTokenToProto(t))
	}

	return connect.NewResponse(&tokenv1.ListTokensResponse{Tokens: protos}), nil
}

func (h *Handler) GetToken(
	ctx context.Context,
	req *connect.Request[tokenv1.GetTokenRequest],
) (*connect.Response[tokenv1.GetTokenResponse], error) {
	token, err := h.uc.Get(ctx, req.Msg.GetId())
	if err != nil {
		return nil, v2.ToConnectError(err)
	}

	return connect.NewResponse(&tokenv1.GetTokenResponse{Token: domainTokenToProto(token)}), nil
}

func (h *Handler) RevokeToken(
	ctx context.Context,
	req *connect.Request[tokenv1.RevokeTokenRequest],
) (*connect.Response[tokenv1.RevokeTokenResponse], error) {
	if err := h.uc.Revoke(ctx, req.Msg.GetId()); err != nil {
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
		Role:       t.Role,
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

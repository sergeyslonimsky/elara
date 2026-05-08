package token

import (
	"context"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/sergeyslonimsky/elara/internal/domain"
	v2 "github.com/sergeyslonimsky/elara/internal/handler/v2"
	tokenv1 "github.com/sergeyslonimsky/elara/internal/proto/elara/token/v1"
	authuc "github.com/sergeyslonimsky/elara/internal/usecase/auth"
)

// Handler implements tokenv1connect.TokenServiceHandler.
type Handler struct {
	create *authuc.CreateTokenUseCase
	list   *authuc.ListTokensUseCase
	get    *authuc.GetTokenUseCase
	revoke *authuc.RevokeTokenUseCase
}

// New returns a new Handler.
func New(
	create *authuc.CreateTokenUseCase,
	list *authuc.ListTokensUseCase,
	get *authuc.GetTokenUseCase,
	revoke *authuc.RevokeTokenUseCase,
) *Handler {
	return &Handler{create: create, list: list, get: get, revoke: revoke}
}

func (h *Handler) CreateToken(
	ctx context.Context,
	req *connect.Request[tokenv1.CreateTokenRequest],
) (*connect.Response[tokenv1.CreateTokenResponse], error) {
	var expiresAt *time.Time
	if req.Msg.GetExpiresAt() != nil {
		expiresAt = new(req.Msg.GetExpiresAt().AsTime())
	}

	token, rawToken, err := h.create.Execute(
		ctx,
		req.Msg.GetName(),
		req.Msg.GetNamespaces(),
		req.Msg.GetRole(),
		expiresAt,
	)
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
	tokens, err := h.list.Execute(ctx, req.Msg.GetIssuedBy())
	if err != nil {
		return nil, v2.ToConnectError(err)
	}

	protos := make([]*tokenv1.Token, 0, len(tokens))
	for _, t := range tokens {
		protos = append(protos, domainTokenToProto(t))
	}

	return connect.NewResponse(&tokenv1.ListTokensResponse{Tokens: protos}), nil
}

func (h *Handler) GetToken(
	ctx context.Context,
	req *connect.Request[tokenv1.GetTokenRequest],
) (*connect.Response[tokenv1.GetTokenResponse], error) {
	token, err := h.get.Execute(ctx, req.Msg.GetId())
	if err != nil {
		return nil, v2.ToConnectError(err)
	}

	return connect.NewResponse(&tokenv1.GetTokenResponse{Token: domainTokenToProto(token)}), nil
}

func (h *Handler) RevokeToken(
	ctx context.Context,
	req *connect.Request[tokenv1.RevokeTokenRequest],
) (*connect.Response[tokenv1.RevokeTokenResponse], error) {
	if err := h.revoke.Execute(ctx, req.Msg.GetId()); err != nil {
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

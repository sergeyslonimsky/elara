package v2

import (
	"context"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/sergeyslonimsky/elara/internal/domain"
	authv1 "github.com/sergeyslonimsky/elara/internal/proto/elara/auth/v1"
	authuc "github.com/sergeyslonimsky/elara/internal/usecase/auth"
)

// TokenHandler implements authv1connect.TokenServiceHandler.
type TokenHandler struct {
	create *authuc.CreateTokenUseCase
	list   *authuc.ListTokensUseCase
	get    *authuc.GetTokenUseCase
	revoke *authuc.RevokeTokenUseCase
}

// NewTokenHandler returns a new TokenHandler.
func NewTokenHandler(
	create *authuc.CreateTokenUseCase,
	list *authuc.ListTokensUseCase,
	get *authuc.GetTokenUseCase,
	revoke *authuc.RevokeTokenUseCase,
) *TokenHandler {
	return &TokenHandler{create: create, list: list, get: get, revoke: revoke}
}

func (h *TokenHandler) CreateToken(
	ctx context.Context,
	req *connect.Request[authv1.CreateTokenRequest],
) (*connect.Response[authv1.CreateTokenResponse], error) {
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
		return nil, toConnectError(err)
	}

	return connect.NewResponse(&authv1.CreateTokenResponse{
		Token:    domainTokenToProto(token),
		RawToken: rawToken,
	}), nil
}

func (h *TokenHandler) ListTokens(
	ctx context.Context,
	req *connect.Request[authv1.ListTokensRequest],
) (*connect.Response[authv1.ListTokensResponse], error) {
	tokens, err := h.list.Execute(ctx, req.Msg.GetIssuedBy())
	if err != nil {
		return nil, toConnectError(err)
	}

	protos := make([]*authv1.Token, 0, len(tokens))
	for _, t := range tokens {
		protos = append(protos, domainTokenToProto(t))
	}

	return connect.NewResponse(&authv1.ListTokensResponse{Tokens: protos}), nil
}

func (h *TokenHandler) GetToken(
	ctx context.Context,
	req *connect.Request[authv1.GetTokenRequest],
) (*connect.Response[authv1.GetTokenResponse], error) {
	token, err := h.get.Execute(ctx, req.Msg.GetId())
	if err != nil {
		return nil, toConnectError(err)
	}

	return connect.NewResponse(&authv1.GetTokenResponse{Token: domainTokenToProto(token)}), nil
}

func (h *TokenHandler) RevokeToken(
	ctx context.Context,
	req *connect.Request[authv1.RevokeTokenRequest],
) (*connect.Response[authv1.RevokeTokenResponse], error) {
	if err := h.revoke.Execute(ctx, req.Msg.GetId()); err != nil {
		return nil, toConnectError(err)
	}

	return connect.NewResponse(&authv1.RevokeTokenResponse{}), nil
}

func domainTokenToProto(t *domain.Token) *authv1.Token {
	if t == nil {
		return nil
	}

	proto := &authv1.Token{
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

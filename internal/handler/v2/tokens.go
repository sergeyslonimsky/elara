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
		t := req.Msg.GetExpiresAt().AsTime()
		expiresAt = &t
	}

	pat, rawToken, err := h.create.Execute(ctx, req.Msg.GetName(), req.Msg.GetNamespaces(), expiresAt)
	if err != nil {
		return nil, toConnectError(err)
	}

	return connect.NewResponse(&authv1.CreateTokenResponse{
		Token:    domainPATToProto(pat),
		RawToken: rawToken,
	}), nil
}

func (h *TokenHandler) ListTokens(
	ctx context.Context,
	req *connect.Request[authv1.ListTokensRequest],
) (*connect.Response[authv1.ListTokensResponse], error) {
	tokens, err := h.list.Execute(ctx, req.Msg.GetUserEmail())
	if err != nil {
		return nil, toConnectError(err)
	}

	protos := make([]*authv1.PAT, 0, len(tokens))
	for _, t := range tokens {
		protos = append(protos, domainPATToProto(t))
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

	return connect.NewResponse(&authv1.GetTokenResponse{Token: domainPATToProto(token)}), nil
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

func domainPATToProto(p *domain.PAT) *authv1.PAT {
	if p == nil {
		return nil
	}

	proto := &authv1.PAT{
		Id:         p.ID,
		Name:       p.Name,
		UserEmail:  p.UserEmail,
		Namespaces: p.Namespaces,
		LastUsedIp: p.LastUsedIP,
		CreatedAt:  timestamppb.New(p.CreatedAt),
	}

	if p.ExpiresAt != nil {
		proto.ExpiresAt = timestamppb.New(*p.ExpiresAt)
	}

	if p.LastUsedAt != nil {
		proto.LastUsedAt = timestamppb.New(*p.LastUsedAt)
	}

	return proto
}

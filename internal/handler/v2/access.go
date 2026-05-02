package v2

import (
	"context"

	"connectrpc.com/connect"

	authv1 "github.com/sergeyslonimsky/elara/internal/proto/elara/auth/v1"
	authuc "github.com/sergeyslonimsky/elara/internal/usecase/auth"
)

// AccessHandler implements authv1connect.AccessServiceHandler.
type AccessHandler struct {
	assign *authuc.AssignRoleUseCase
	revoke *authuc.RevokeRoleUseCase
	list   *authuc.ListPoliciesUseCase
}

// NewAccessHandler returns a new AccessHandler.
func NewAccessHandler(
	assign *authuc.AssignRoleUseCase,
	revoke *authuc.RevokeRoleUseCase,
	list *authuc.ListPoliciesUseCase,
) *AccessHandler {
	return &AccessHandler{assign: assign, revoke: revoke, list: list}
}

func (h *AccessHandler) AssignRole(
	ctx context.Context,
	req *connect.Request[authv1.AssignRoleRequest],
) (*connect.Response[authv1.AssignRoleResponse], error) {
	if err := h.assign.Execute(ctx, req.Msg.GetSubject(), req.Msg.GetDomain(), req.Msg.GetRole()); err != nil {
		return nil, toConnectError(err)
	}

	return connect.NewResponse(&authv1.AssignRoleResponse{}), nil
}

func (h *AccessHandler) RevokeRole(
	ctx context.Context,
	req *connect.Request[authv1.RevokeRoleRequest],
) (*connect.Response[authv1.RevokeRoleResponse], error) {
	if err := h.revoke.Execute(ctx, req.Msg.GetSubject(), req.Msg.GetDomain(), req.Msg.GetRole()); err != nil {
		return nil, toConnectError(err)
	}

	return connect.NewResponse(&authv1.RevokeRoleResponse{}), nil
}

func (h *AccessHandler) ListPolicies(
	ctx context.Context,
	_ *connect.Request[authv1.ListPoliciesRequest],
) (*connect.Response[authv1.ListPoliciesResponse], error) {
	rules, err := h.list.Execute(ctx)
	if err != nil {
		return nil, toConnectError(err)
	}

	protos := make([]*authv1.PolicyRule, 0, len(rules))
	for _, r := range rules {
		protos = append(protos, &authv1.PolicyRule{
			Subject: r.Subject,
			Domain:  r.Domain,
			Role:    r.Role,
		})
	}

	return connect.NewResponse(&authv1.ListPoliciesResponse{Rules: protos}), nil
}

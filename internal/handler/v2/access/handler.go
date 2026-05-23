package access

import (
	"context"

	"connectrpc.com/connect"

	"github.com/sergeyslonimsky/elara/internal/domain"
	v2 "github.com/sergeyslonimsky/elara/internal/handler/v2"
	accessv1 "github.com/sergeyslonimsky/elara/internal/proto/elara/access/v1"
	policyuc "github.com/sergeyslonimsky/elara/internal/usecase/policy"
)

//go:generate mockgen -destination=mocks/handler_mock.go -package=access_mock -source=handler.go

type (
	authz interface {
		Require(ctx context.Context, object, action, domainStr string) error
	}

	accessUsecase interface {
		AssignRole(ctx context.Context, subject, dom, role string) error
		RevokeRole(ctx context.Context, subject, dom, role string) error
		List(ctx context.Context) ([]policyuc.Rule, error)
	}
)

// AccessHandler implements accessv1connect.AccessServiceHandler.
type AccessHandler struct {
	authz authz
	uc    accessUsecase
}

// NewAccessHandler returns a new AccessHandler.
func NewAccessHandler(authz authz, uc accessUsecase) *AccessHandler {
	return &AccessHandler{authz: authz, uc: uc}
}

func (h *AccessHandler) AssignRole(
	ctx context.Context,
	req *connect.Request[accessv1.AssignRoleRequest],
) (*connect.Response[accessv1.AssignRoleResponse], error) {
	if err := h.authz.Require(ctx, domain.ObjectPolicy, domain.ActionWrite, domain.DomainAll); err != nil {
		return nil, v2.ToConnectError(err)
	}

	if err := h.uc.AssignRole(ctx, req.Msg.GetSubject(), req.Msg.GetDomain(), req.Msg.GetRole()); err != nil {
		return nil, v2.ToConnectError(err)
	}

	return connect.NewResponse(&accessv1.AssignRoleResponse{}), nil
}

func (h *AccessHandler) RevokeRole(
	ctx context.Context,
	req *connect.Request[accessv1.RevokeRoleRequest],
) (*connect.Response[accessv1.RevokeRoleResponse], error) {
	if err := h.authz.Require(ctx, domain.ObjectPolicy, domain.ActionWrite, domain.DomainAll); err != nil {
		return nil, v2.ToConnectError(err)
	}

	if err := h.uc.RevokeRole(ctx, req.Msg.GetSubject(), req.Msg.GetDomain(), req.Msg.GetRole()); err != nil {
		return nil, v2.ToConnectError(err)
	}

	return connect.NewResponse(&accessv1.RevokeRoleResponse{}), nil
}

func (h *AccessHandler) ListPolicies(
	ctx context.Context,
	_ *connect.Request[accessv1.ListPoliciesRequest],
) (*connect.Response[accessv1.ListPoliciesResponse], error) {
	rules, err := h.uc.List(ctx)
	if err != nil {
		return nil, v2.ToConnectError(err)
	}

	protos := make([]*accessv1.PolicyRule, 0, len(rules))
	for _, r := range rules {
		protos = append(protos, &accessv1.PolicyRule{
			Subject: r.Subject,
			Domain:  r.Domain,
			Role:    r.Role,
		})
	}

	return connect.NewResponse(&accessv1.ListPoliciesResponse{Rules: protos}), nil
}

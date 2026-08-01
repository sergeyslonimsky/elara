package capabilities

import (
	"context"

	"connectrpc.com/connect"

	capabilitiesv1 "github.com/sergeyslonimsky/elara/internal/proto/elara/capabilities/v1"
)

// Handler implements capabilitiesv1connect.CapabilitiesServiceHandler.
type Handler struct {
	etcdTokenAuthEnabled  bool
	userManagementEnabled bool
}

// New returns a new Handler wired with the conditionally-mounted feature flags.
func New(etcdTokenAuthEnabled, userManagementEnabled bool) *Handler {
	return &Handler{
		etcdTokenAuthEnabled:  etcdTokenAuthEnabled,
		userManagementEnabled: userManagementEnabled,
	}
}

func (h *Handler) GetCapabilities(
	_ context.Context,
	_ *connect.Request[capabilitiesv1.GetCapabilitiesRequest],
) (*connect.Response[capabilitiesv1.GetCapabilitiesResponse], error) {
	return connect.NewResponse(&capabilitiesv1.GetCapabilitiesResponse{
		EtcdTokenAuthEnabled:  h.etcdTokenAuthEnabled,
		UserManagementEnabled: h.userManagementEnabled,
	}), nil
}

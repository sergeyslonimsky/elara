package capabilities_test

import (
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/internal/handler/v2/capabilities"
	capabilitiesv1 "github.com/sergeyslonimsky/elara/internal/proto/elara/capabilities/v1"
)

func TestHandler_GetCapabilities(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                  string
		etcdTokenAuthEnabled  bool
		userManagementEnabled bool
		demoMode              bool
	}{
		{
			name:                  "both enabled",
			etcdTokenAuthEnabled:  true,
			userManagementEnabled: true,
			demoMode:              false,
		},
		{
			name:                  "etcd token auth enabled, user management disabled",
			etcdTokenAuthEnabled:  true,
			userManagementEnabled: false,
			demoMode:              false,
		},
		{
			name:                  "etcd token auth disabled, user management enabled",
			etcdTokenAuthEnabled:  false,
			userManagementEnabled: true,
			demoMode:              false,
		},
		{
			name:                  "both disabled",
			etcdTokenAuthEnabled:  false,
			userManagementEnabled: false,
			demoMode:              false,
		},
		{
			name:                  "demo mode enabled",
			etcdTokenAuthEnabled:  false,
			userManagementEnabled: true,
			demoMode:              true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			h := capabilities.New(tc.etcdTokenAuthEnabled, tc.userManagementEnabled, tc.demoMode)

			resp, err := h.GetCapabilities(
				t.Context(),
				connect.NewRequest(&capabilitiesv1.GetCapabilitiesRequest{}),
			)

			require.NoError(t, err)
			assert.Equal(t, tc.etcdTokenAuthEnabled, resp.Msg.GetEtcdTokenAuthEnabled())
			assert.Equal(t, tc.userManagementEnabled, resp.Msg.GetUserManagementEnabled())
			assert.Equal(t, tc.demoMode, resp.Msg.GetDemoMode())
		})
	}
}

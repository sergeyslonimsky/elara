package internal_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/sergeyslonimsky/elara/internal/domain"
	storageinternal "github.com/sergeyslonimsky/elara/internal/storage/internal"
)

func TestDomainToClientHistoryRow(t *testing.T) {
	t.Parallel()

	now := time.Now()
	disconnectedAt := now.Add(time.Minute)

	tests := []struct {
		name   string
		client *domain.Client
		want   storageinternal.ClientHistoryRow
	}{
		{
			name: "full client",
			client: &domain.Client{
				ID:             "client-1",
				PeerAddress:    "1.2.3.4:5678",
				UserAgent:      "ua",
				ClientName:     "cli",
				ClientVersion:  "1.0.0",
				K8sNamespace:   "default",
				K8sPod:         "pod-1",
				K8sNode:        "node-1",
				InstanceID:     "inst-1",
				ConnectedAt:    now,
				DisconnectedAt: &disconnectedAt,
				LastActivityAt: now,
				ActiveWatches:  2,
				RequestCounts:  map[string]int64{"get": 3},
				ErrorCount:     1,
			},
			want: storageinternal.ClientHistoryRow{
				ID:             "client-1",
				PeerAddress:    "1.2.3.4:5678",
				UserAgent:      "ua",
				ClientName:     "cli",
				ClientVersion:  "1.0.0",
				K8sNamespace:   "default",
				K8sPod:         "pod-1",
				K8sNode:        "node-1",
				InstanceID:     "inst-1",
				ConnectedAt:    now,
				DisconnectedAt: disconnectedAt,
				LastActivityAt: now,
				ActiveWatches:  2,
				RequestCounts:  map[string]int64{"get": 3},
				ErrorCount:     1,
			},
		},
		{
			name: "zero value client with non-nil DisconnectedAt",
			client: &domain.Client{
				DisconnectedAt: &time.Time{},
			},
			want: storageinternal.ClientHistoryRow{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := storageinternal.DomainToClientHistoryRow(tt.client)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestClientHistoryRowToDomain(t *testing.T) {
	t.Parallel()

	now := time.Now()
	disconnectedAt := now.Add(time.Minute)

	tests := []struct {
		name string
		row  storageinternal.ClientHistoryRow
		want *domain.Client
	}{
		{
			name: "full row",
			row: storageinternal.ClientHistoryRow{
				ID:             "client-1",
				PeerAddress:    "1.2.3.4:5678",
				UserAgent:      "ua",
				ClientName:     "cli",
				ClientVersion:  "1.0.0",
				K8sNamespace:   "default",
				K8sPod:         "pod-1",
				K8sNode:        "node-1",
				InstanceID:     "inst-1",
				ConnectedAt:    now,
				DisconnectedAt: disconnectedAt,
				LastActivityAt: now,
				ActiveWatches:  2,
				RequestCounts:  map[string]int64{"get": 3},
				ErrorCount:     1,
			},
			want: &domain.Client{
				ID:             "client-1",
				PeerAddress:    "1.2.3.4:5678",
				UserAgent:      "ua",
				ClientName:     "cli",
				ClientVersion:  "1.0.0",
				K8sNamespace:   "default",
				K8sPod:         "pod-1",
				K8sNode:        "node-1",
				InstanceID:     "inst-1",
				ConnectedAt:    now,
				DisconnectedAt: &disconnectedAt,
				LastActivityAt: now,
				ActiveWatches:  2,
				RequestCounts:  map[string]int64{"get": 3},
				ErrorCount:     1,
			},
		},
		{
			name: "zero value row",
			row:  storageinternal.ClientHistoryRow{},
			want: &domain.Client{
				DisconnectedAt: &time.Time{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := storageinternal.ClientHistoryRowToDomain(tt.row)
			assert.Equal(t, tt.want, got)
		})
	}
}

package config

import (
	"time"

	"github.com/sergeyslonimsky/core/di"
	coregrpc "github.com/sergeyslonimsky/core/grpc"
)

type Client struct {
	EtcdServer   coregrpc.Config
	Auth         ClientAuth
	History      ClientHistory
	RecentEvents ClientRecentEvents
}

type ClientHistory struct {
	MaxRecords int
	MaxAge     time.Duration
}

type ClientRecentEvents struct {
	Capacity int
}

type ClientAuth struct {
	Enabled bool
}

func newClientConfig(cfg *di.Config) Client {
	return Client{
		EtcdServer: coregrpc.Config{
			Port: di.GetOrDefault(cfg, "client.etcd.port", defaultGRPCPort),
		},
		Auth: ClientAuth{
			Enabled: di.Get[bool](cfg, "client.auth.enabled"),
		},
		History: ClientHistory{
			MaxRecords: intOrDefault(
				di.Get[int](cfg, "client.history.max_records"),
				defaultClientHistoryMaxRecords,
			),
			MaxAge: durOrDefault(
				di.Get[time.Duration](cfg, "client.history.max_age"),
				defaultClientHistoryMaxAge,
			),
		},
		RecentEvents: ClientRecentEvents{
			Capacity: intOrDefault(
				di.Get[int](cfg, "client.recent_events.capacity"),
				defaultClientRecentEventsCap,
			),
		},
	}
}

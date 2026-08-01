// Package watcher holds the tiny bit of logic every toy service in this demo
// shares: connect to Elara over the etcd v3 wire protocol, read one config key
// on startup, then watch it and print whenever it changes.
//
// This is deliberately minimal demo code — no DDD layering, no graceful
// production concerns. The point is to show that a plain go.etcd.io/etcd
// client, unmodified, talks to Elara exactly as it would to real etcd.
package watcher

import (
	"context"
	"log"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
)

// Config describes what a single service watches.
type Config struct {
	// Service is a human label used only for log lines (e.g. "api").
	Service string
	// Endpoint is the Elara etcd-compatible gRPC address, e.g. "elara:2379".
	Endpoint string
	// Key is the full etcd key: "/{namespace}/{path}", e.g.
	// "/production/api/limits.json".
	Key string
	// Fallback is printed when the key does not exist yet, so the service
	// stays useful before anyone has created the config in the UI.
	Fallback string
}

// Run connects (retrying until Elara is reachable), prints the current value of
// the key, then blocks watching for changes until ctx is cancelled.
func Run(ctx context.Context, cfg Config) {
	cli := connect(ctx, cfg)
	if cli == nil {
		return
	}
	defer func() { _ = cli.Close() }()

	readInitial(ctx, cli, cfg)
	watch(ctx, cli, cfg)
}

func connect(ctx context.Context, cfg Config) *clientv3.Client {
	for {
		if ctx.Err() != nil {
			return nil
		}

		cli, err := clientv3.New(clientv3.Config{
			Endpoints:   []string{cfg.Endpoint},
			DialTimeout: 5 * time.Second,
			Context:     ctx,
		})
		if err == nil {
			return cli
		}

		log.Printf("[%s] cannot reach Elara at %s (%v) — retrying in 3s", cfg.Service, cfg.Endpoint, err)
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(3 * time.Second):
		}
	}
}

func readInitial(ctx context.Context, cli *clientv3.Client, cfg Config) {
	getCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	resp, err := cli.Get(getCtx, cfg.Key)
	if err != nil {
		log.Printf("[%s] initial read of %s failed (%v) — using fallback", cfg.Service, cfg.Key, err)
		log.Printf("[%s] config %s = %s (fallback)", cfg.Service, cfg.Key, cfg.Fallback)

		return
	}

	if len(resp.Kvs) == 0 {
		log.Printf("[%s] %s not set yet — using fallback %s", cfg.Service, cfg.Key, cfg.Fallback)

		return
	}

	log.Printf("[%s] loaded %s = %s", cfg.Service, cfg.Key, string(resp.Kvs[0].Value))
}

func watch(ctx context.Context, cli *clientv3.Client, cfg Config) {
	log.Printf("[%s] watching %s for changes...", cfg.Service, cfg.Key)

	wch := cli.Watch(ctx, cfg.Key)
	for wr := range wch {
		if err := wr.Err(); err != nil {
			log.Printf("[%s] watch error: %v", cfg.Service, err)

			continue
		}

		for _, ev := range wr.Events {
			switch ev.Type {
			case clientv3.EventTypePut:
				log.Printf("[%s] config changed: %s = %s (revision %d)",
					cfg.Service, cfg.Key, string(ev.Kv.Value), ev.Kv.ModRevision)
			case clientv3.EventTypeDelete:
				log.Printf("[%s] config deleted: %s — reverting to fallback %s",
					cfg.Service, cfg.Key, cfg.Fallback)
			}
		}
	}
}

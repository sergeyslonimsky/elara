// Command notifier is a toy notification service. It reads its channel config
// from Elara over the etcd v3 protocol and reacts to changes live.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/sergeyslonimsky/elara/examples/todo-app/watcher"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	watcher.Run(ctx, watcher.Config{
		Service:  "notifier",
		Endpoint: env("ELARA_ENDPOINT", "elara:2379"),
		Key:      env("CONFIG_KEY", "/production/notifier/config.json"),
		Fallback: `{"channel":"email","digest_enabled":true}`,
	})
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}

	return fallback
}

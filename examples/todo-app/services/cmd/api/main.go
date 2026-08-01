// Command api is a toy "todo API" service. It reads its limits config from
// Elara over the etcd v3 protocol and reacts to changes live.
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
		Service:  "api",
		Endpoint: env("ELARA_ENDPOINT", "elara:2379"),
		Key:      env("CONFIG_KEY", "/production/api/limits.json"),
		Fallback: `{"max_todos_per_user":100,"rate_limit_per_min":60}`,
	})
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}

	return fallback
}

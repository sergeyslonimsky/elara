// Command seed is a one-shot container that writes the demo's initial config
// keys into Elara over the etcd v3 protocol, so the api/worker/notifier
// services show real values on first boot instead of only fallbacks.
//
// It is idempotent: re-running it simply overwrites the same keys.
package main

import (
	"context"
	"log"
	"os"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
)

func main() {
	endpoint := env("ELARA_ENDPOINT", "elara:2379")

	seeds := map[string]string{
		"/production/api/limits.json":      `{"max_todos_per_user":100,"rate_limit_per_min":60}`,
		"/production/worker/settings.json": `{"batch_size":50,"poll_interval_seconds":5}`,
		"/production/notifier/config.json": `{"channel":"email","digest_enabled":true}`,
	}

	cli := connect(endpoint)
	defer func() { _ = cli.Close() }()

	for key, val := range seeds {
		putCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		if _, err := cli.Put(putCtx, key, val); err != nil {
			cancel()
			log.Fatalf("seed put %s: %v", key, err)
		}
		cancel()
		log.Printf("seeded %s = %s", key, val)
	}

	log.Println("seed complete")
}

func connect(endpoint string) *clientv3.Client {
	// Elara may still be starting; retry until the dial succeeds.
	for attempt := 1; attempt <= 20; attempt++ {
		cli, err := clientv3.New(clientv3.Config{
			Endpoints:   []string{endpoint},
			DialTimeout: 5 * time.Second,
		})
		if err == nil {
			// Verify the server actually answers before returning.
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			_, gerr := cli.Get(ctx, "/production")
			cancel()
			if gerr == nil {
				return cli
			}
			_ = cli.Close()
			err = gerr
		}

		log.Printf("waiting for Elara at %s (%v) — attempt %d/20", endpoint, err, attempt)
		time.Sleep(3 * time.Second)
	}

	log.Fatalf("Elara never became reachable at %s", endpoint)

	return nil
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}

	return fallback
}

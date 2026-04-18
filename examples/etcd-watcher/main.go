// Demo etcd client that connects to a Elara instance, watches keys, and
// periodically writes a heartbeat. Designed to verify the connected-clients UI.
//
// Usage:
//
//	cd examples/etcd-watcher
//	go mod tidy
//	go run .
//
// Common scenarios:
//
//	# Default: prefix watch on /default/, periodic Put+Get every 5s
//	go run .
//
//	# Single-key watch on a specific config (verifies Watches tab "key" badge)
//	go run . --watch-mode key --watch-key /default/demo/heartbeat.json
//
//	# Explicit range watch
//	go run . --watch-mode range --watch-key /default/a --watch-end /default/m
//
//	# Generate errors so the Errors tab populates
//	go run . --errors-every 3   # every 3rd request fails
//
//	# Multiple distinct clients
//	go run . --name payment-service --pod payment-abc
//	go run . --name order-service --pod order-7d8c --interval 2s
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc/metadata"
)

func main() {
	var (
		endpoint     = flag.String("endpoint", "localhost:2379", "Elara etcd-compatible endpoint")
		name         = flag.String("name", "order-service", "x-client-name")
		version      = flag.String("version", "1.2.3", "x-client-version")
		k8sNamespace = flag.String("k8s-ns", "production", "x-client-k8s-namespace")
		k8sPod       = flag.String("pod", "order-7d8c-x4k2", "x-client-k8s-pod")
		k8sNode      = flag.String("k8s-node", "gke-node-abc", "x-client-k8s-node")
		instanceID   = flag.String("instance-id", "instance-uuid-1", "x-client-instance-id")

		watchMode = flag.String("watch-mode", "prefix", "watch mode: prefix | key | range | none")
		watchKey  = flag.String("watch-key", "/default/", "key to watch (start key for range)")
		watchEnd  = flag.String("watch-end", "", "end key — used only for --watch-mode=range")

		writePath    = flag.String("write-path", "/default/demo/heartbeat.json", "key written every tick")
		readPath     = flag.String("read-path", "", "key GET'd every tick (defaults to write-path)")
		interval     = flag.Duration("interval", 5*time.Second, "heartbeat interval (0 = disable writes)")
		errorsEveryN = flag.Int("errors-every", 0, "every Nth request fails on purpose (0 = no errors)")
	)
	flag.Parse()

	log.SetFlags(log.Ltime)

	if *readPath == "" {
		*readPath = *writePath
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{*endpoint},
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		log.Fatalf("connect %s: %v", *endpoint, err)
	}
	defer cli.Close()

	log.Printf("connected to %s", *endpoint)
	log.Printf("identity: name=%s version=%s k8s=%s/%s instance=%s",
		*name, *version, *k8sNamespace, *k8sPod, *instanceID)

	// Every RPC from this client carries these headers — they show up in the
	// Elara UI under /clients/:id as ClientName, ClientVersion, K8sNamespace, etc.
	md := metadata.AppendToOutgoingContext(ctx,
		"x-client-name", *name,
		"x-client-version", *version,
		"x-client-k8s-namespace", *k8sNamespace,
		"x-client-k8s-pod", *k8sPod,
		"x-client-k8s-node", *k8sNode,
		"x-client-instance-id", *instanceID,
	)

	// Open the watch *before* the writer so we don't miss our own first event.
	wch, err := openWatch(md, cli, *watchMode, *watchKey, *watchEnd)
	if err != nil {
		log.Fatalf("watch setup: %v", err)
	}

	if *interval > 0 {
		go writer(md, cli, *writePath, *readPath, *interval, *errorsEveryN)
	}

	// Main loop: print every watch event.
	for {
		select {
		case <-ctx.Done():
			log.Println("shutting down")
			return

		case wresp, ok := <-wch:
			if !ok {
				log.Println("watch channel closed by server")
				return
			}

			if err := wresp.Err(); err != nil {
				log.Printf("watch error: %v", err)
				continue
			}

			for _, ev := range wresp.Events {
				log.Printf("WATCH %s key=%s rev=%d value=%s",
					ev.Type, ev.Kv.Key, ev.Kv.ModRevision, truncate(ev.Kv.Value, 80))
			}
		}
	}
}

func openWatch(ctx context.Context, cli *clientv3.Client, mode, key, end string) (clientv3.WatchChan, error) {
	switch mode {
	case "none":
		log.Printf("watch disabled")
		return nil, nil

	case "key":
		log.Printf("watching single key %q", key)
		return cli.Watch(ctx, key), nil

	case "prefix":
		log.Printf("watching prefix %q", key)
		return cli.Watch(ctx, key, clientv3.WithPrefix()), nil

	case "range":
		if end == "" {
			return nil, fmt.Errorf("--watch-end required for range mode")
		}
		log.Printf("watching range [%q, %q)", key, end)
		return cli.Watch(ctx, key, clientv3.WithRange(end)), nil

	default:
		return nil, fmt.Errorf("unknown --watch-mode %q (valid: prefix|key|range|none)", mode)
	}
}

// writer periodically does Put + Get so the per-method counters in the UI grow
// for both KV.Put and KV.Range. When errorsEveryN > 0, every Nth iteration
// fires a deliberately-failing Put (using IgnoreValue, which our server returns
// Unimplemented for) so the Errors tab populates.
func writer(
	ctx context.Context,
	cli *clientv3.Client,
	writePath, readPath string,
	interval time.Duration,
	errorsEveryN int,
) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	counter := 0
	for {
		select {
		case <-ctx.Done():
			return

		case <-ticker.C:
			counter++

			value := fmt.Sprintf(`{"counter":%d,"timestamp":%q,"pid":%d}`,
				counter, time.Now().UTC().Format(time.RFC3339), os.Getpid())

			if _, err := cli.Put(ctx, writePath, value); err != nil {
				log.Printf("PUT failed: %v", err)
				continue
			}

			log.Printf("PUT %s counter=%d", writePath, counter)

			if resp, err := cli.Get(ctx, readPath); err != nil {
				log.Printf("GET failed: %v", err)
			} else if resp.Count > 0 {
				log.Printf("GET %s rev=%d", readPath, resp.Header.Revision)
			}

			if errorsEveryN > 0 && counter%errorsEveryN == 0 {
				// IgnoreValue is documented as unsupported on our server →
				// returns gRPC Unimplemented, recorded as an error event.
				_, err := cli.Put(ctx, writePath, "x", clientv3.WithIgnoreValue())
				if err != nil {
					log.Printf("ERR (intentional) %v", err)
				}
			}
		}
	}
}

func truncate(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}

	return string(b[:n]) + "..."
}

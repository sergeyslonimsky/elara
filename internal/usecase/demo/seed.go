// Package demo seeds an Elara instance with sample data for the `elara:demo`
// image. It reuses the real usecase layer (never touching bbolt directly) to
// create namespaces, configs, and JSON Schema attachments, and injects
// simulated etcd clients into the connected-clients monitor.
//
// Seeding is idempotent for persistent data (namespaces/configs/schemas) and
// runs on every startup for the in-memory client snapshots, which are not
// persisted and therefore vanish on restart.
package demo

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/sergeyslonimsky/elara/internal/domain"
	configuc "github.com/sergeyslonimsky/elara/internal/usecase/config"
	nsuc "github.com/sergeyslonimsky/elara/internal/usecase/namespace"
	schemauc "github.com/sergeyslonimsky/elara/internal/usecase/schema"
)

// clientRegistry is the subset of the connected-clients monitor demo seeding
// drives to inject simulated etcd consumers. Satisfied by *monitor.Registry.
type clientRegistry interface {
	RegisterConnection(info domain.ConnectionInfo) string
	RegisterWatch(connID string, w domain.ActiveWatch)
	RecordRequest(connID, method, key string, revision int64, duration time.Duration, err error)
}

// Deps carries the collaborators demo seeding drives — the concrete usecase
// services and the client monitor already wired in DI. demo is a bootstrap
// helper: it constructs nothing and owns no transactions.
type Deps struct {
	Namespaces *nsuc.Service
	Configs    *configuc.Service
	Schemas    *schemauc.Service
	Clients    clientRegistry
}

// Seed populates the instance with sample data. Persistent data is written only
// when the instance has not been seeded before; simulated clients are injected
// on every call because the monitor is in-memory.
func Seed(ctx context.Context, deps Deps) error {
	seeded, err := alreadySeeded(ctx, deps.Namespaces)
	if err != nil {
		return fmt.Errorf("check seed state: %w", err)
	}

	if seeded {
		slog.InfoContext(ctx, "demo: sample data already present, skipping persistent seed")
	} else {
		if err := seedNamespaces(ctx, deps.Namespaces); err != nil {
			return err
		}

		if err := seedSchemas(ctx, deps.Schemas); err != nil {
			return err
		}

		if err := seedConfigs(ctx, deps.Configs); err != nil {
			return err
		}

		slog.InfoContext(ctx, "demo: seeded sample namespaces, configs, and schemas")
	}

	seedClients(deps.Clients)
	slog.InfoContext(ctx, "demo: injected simulated etcd clients into monitor")

	return nil
}

// alreadySeeded reports whether the sample data is present, keyed on the
// production namespace. A missing namespace means a fresh instance.
func alreadySeeded(ctx context.Context, namespaces *nsuc.Service) (bool, error) {
	_, err := namespaces.Get(ctx, nsProduction)
	if err == nil {
		return true, nil
	}

	if errors.Is(err, domain.ErrNotFound) {
		return false, nil
	}

	return false, fmt.Errorf("get %q namespace: %w", nsProduction, err)
}

func seedNamespaces(ctx context.Context, namespaces *nsuc.Service) error {
	for _, ns := range sampleNamespaces {
		_, err := namespaces.Create(ctx, &domain.Namespace{
			Name:        ns.name,
			DisplayName: ns.displayName,
			Description: ns.description,
		})
		if err != nil {
			return fmt.Errorf("create namespace %q: %w", ns.name, err)
		}
	}

	return nil
}

func seedSchemas(ctx context.Context, schemas *schemauc.Service) error {
	for _, s := range sampleSchemas {
		_, err := schemas.Attach(ctx, schemauc.AttachInput{
			Namespace:   s.namespace,
			PathPattern: s.pathPattern,
			JSONSchema:  s.jsonSchema,
		})
		if err != nil {
			return fmt.Errorf("attach schema %q%q: %w", s.namespace, s.pathPattern, err)
		}
	}

	return nil
}

func seedConfigs(ctx context.Context, configs *configuc.Service) error {
	for _, c := range sampleConfigs {
		_, err := configs.Create(ctx, &domain.Config{
			Namespace: c.namespace,
			Path:      c.path,
			Content:   c.content,
		})
		if err != nil {
			return fmt.Errorf("create config %q%q: %w", c.namespace, c.path, err)
		}
	}

	return nil
}

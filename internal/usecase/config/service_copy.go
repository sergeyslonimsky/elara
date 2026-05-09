package config

import (
	"context"
	"errors"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/auth"
	"github.com/sergeyslonimsky/elara/internal/util/maputil"
)

type CopyInput struct {
	SourcePath      string
	SourceNamespace string
	DestPath        string
	DestNamespace   string
}

func (s *Service) Copy(ctx context.Context, in CopyInput) (*domain.Config, error) {
	if err := domain.ValidatePath(in.DestPath); err != nil {
		return nil, fmt.Errorf("validate destination path: %w", err)
	}

	if in.SourceNamespace == "" {
		return nil, domain.NewValidationError("source_namespace", "namespace is required")
	}

	if in.DestNamespace == "" {
		return nil, domain.NewValidationError("destination_namespace", "namespace is required")
	}

	// Check write permission on destination namespace.
	if err := auth.CheckAccess(ctx, s.enforcer, in.DestNamespace, auth.ObjectConfig, auth.ActionWrite); err != nil {
		return nil, fmt.Errorf("check access: %w", err)
	}

	// Get source config.
	source, err := s.storage.Get(ctx, in.SourcePath, in.SourceNamespace)
	if err != nil {
		return nil, fmt.Errorf("get source config: %w", err)
	}

	// Check destination namespace exists and is not locked.
	dstNs, err := s.namespaceProvider.Get(ctx, in.DestNamespace)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, domain.NewValidationError(
				"destination_namespace",
				fmt.Sprintf("namespace %q does not exist", in.DestNamespace),
			)
		}

		return nil, fmt.Errorf("check destination namespace: %w", err)
	}

	if dstNs.Locked {
		return nil, fmt.Errorf("destination namespace %q: %w", in.DestNamespace, domain.ErrLocked)
	}

	// Create copy at destination.
	dest := &domain.Config{
		Path:      in.DestPath,
		Content:   source.Content,
		Format:    source.Format,
		Namespace: in.DestNamespace,
		Metadata:  maputil.Clone(source.Metadata),
	}

	dest.GenerateHash()
	dest.Version = 1

	if err := s.storage.Create(ctx, dest); err != nil {
		return nil, fmt.Errorf("create copy: %w", err)
	}

	// best-effort: namespace timestamp is cosmetic; failure must not abort the config write.
	_ = s.namespaceProvider.UpdateTimestamp(ctx, in.DestNamespace)
	s.watcher.NotifyCreated(ctx, dest)

	return dest, nil
}

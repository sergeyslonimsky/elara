package schema

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/auth"
)

type AttachInput struct {
	Namespace   string
	PathPattern string
	JSONSchema  string
}

func (s *Service) Attach(ctx context.Context, in AttachInput) (*domain.SchemaAttachment, error) {
	claims, ok := auth.ClaimsFromContext(ctx)
	if !ok {
		return nil, domain.ErrUnauthorized
	}

	allowed, err := s.enforcer.Enforce(claims.Email, in.Namespace, "schema", "write")
	if err != nil {
		return nil, fmt.Errorf("enforce: %w", err)
	}

	if !allowed {
		return nil, domain.ErrForbidden
	}

	ns, err := s.namespaces.Get(ctx, in.Namespace)
	if err != nil {
		return nil, fmt.Errorf("get namespace: %w", err)
	}

	if ns.Locked {
		return nil, fmt.Errorf("namespace %q: %w", in.Namespace, domain.ErrNamespaceLocked)
	}

	if err := domain.ValidateJSONSchema(in.JSONSchema); err != nil {
		return nil, fmt.Errorf("validate json schema: %w", err)
	}

	attachment := &domain.SchemaAttachment{
		ID:          uuid.New().String(),
		Namespace:   in.Namespace,
		PathPattern: in.PathPattern,
		JSONSchema:  in.JSONSchema,
		CreatedAt:   time.Now(),
	}

	if err := s.store.Attach(ctx, attachment); err != nil {
		return nil, fmt.Errorf("attach schema: %w", err)
	}

	return attachment, nil
}

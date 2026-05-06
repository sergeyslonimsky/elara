package schema

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/sergeyslonimsky/elara/internal/auth"
	"github.com/sergeyslonimsky/elara/internal/domain"
)

//go:generate mockgen -destination=mocks/mock_attach.go -package=schema_mock . attachEnforcer,schemaAttacher,attachNSChecker

type attachEnforcer interface {
	Enforce(subject, domain, object, action string) (bool, error)
}

type schemaAttacher interface {
	Attach(ctx context.Context, s *domain.SchemaAttachment) error
}

type attachNSChecker interface {
	Get(ctx context.Context, name string) (*domain.Namespace, error)
}

type AttachUseCase struct {
	enforcer   attachEnforcer
	store      schemaAttacher
	namespaces attachNSChecker
}

func NewAttachUseCase(enforcer attachEnforcer, store schemaAttacher, namespaces attachNSChecker) *AttachUseCase {
	return &AttachUseCase{enforcer: enforcer, store: store, namespaces: namespaces}
}

func (uc *AttachUseCase) Execute(
	ctx context.Context,
	namespace, pathPattern, jsonSchema string,
) (*domain.SchemaAttachment, error) {
	claims, ok := auth.ClaimsFromContext(ctx)
	if !ok {
		return nil, domain.ErrUnauthorized
	}

	allowed, err := uc.enforcer.Enforce(claims.Email, namespace, "schema", "write")
	if err != nil {
		return nil, fmt.Errorf("enforce: %w", err)
	}

	if !allowed {
		return nil, domain.ErrForbidden
	}

	ns, err := uc.namespaces.Get(ctx, namespace)
	if err != nil {
		return nil, fmt.Errorf("get namespace: %w", err)
	}

	if ns.Locked {
		return nil, fmt.Errorf("namespace %q: %w", namespace, domain.ErrNamespaceLocked)
	}

	if err := domain.ValidateJSONSchema(jsonSchema); err != nil {
		return nil, fmt.Errorf("validate json schema: %w", err)
	}

	s := &domain.SchemaAttachment{
		ID:          uuid.New().String(),
		Namespace:   namespace,
		PathPattern: pathPattern,
		JSONSchema:  jsonSchema,
		CreatedAt:   time.Now(),
	}

	if err := uc.store.Attach(ctx, s); err != nil {
		return nil, fmt.Errorf("attach schema: %w", err)
	}

	return s, nil
}

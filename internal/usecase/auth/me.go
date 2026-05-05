package auth

import (
	"context"
	"fmt"
	"sort"

	"github.com/sergeyslonimsky/elara/internal/auth"
	"github.com/sergeyslonimsky/elara/internal/domain"
)

// NamespaceAccess describes a namespace the current user can see and whether they can write to it.
type NamespaceAccess struct {
	Name     string
	CanWrite bool
}

// MeResult holds the resolved identity and permission summary for the current user.
type MeResult struct {
	Email             string
	Name              string
	IsAdmin           bool
	Namespaces        []NamespaceAccess
	CanViewWebhooks   bool
	CanManageWebhooks bool
}

type meEnforcer interface {
	Enforce(subject, domain, object, action string) (bool, error)
}

type meNamespaceLister interface {
	List(ctx context.Context) ([]*domain.Namespace, error)
}

// MeUseCase returns the current authenticated user's identity and permissions.
type MeUseCase struct {
	enforcer   meEnforcer
	namespaces meNamespaceLister
}

// NewMeUseCase returns a MeUseCase backed by the given enforcer and namespace lister.
func NewMeUseCase(enforcer meEnforcer, namespaces meNamespaceLister) *MeUseCase {
	return &MeUseCase{enforcer: enforcer, namespaces: namespaces}
}

// Execute extracts claims from the context and returns the user's identity and permissions.
func (uc *MeUseCase) Execute(ctx context.Context) (*MeResult, error) {
	claims, ok := auth.ClaimsFromContext(ctx)
	if !ok {
		return nil, domain.ErrUnauthorized
	}

	allNamespaces, err := uc.namespaces.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list namespaces: %w", err)
	}

	var accessible []NamespaceAccess
	for _, ns := range allNamespaces {
		canRead, _ := uc.enforcer.Enforce(claims.Email, ns.Name, auth.ObjectConfig, auth.ActionRead)
		if !canRead {
			continue
		}
		canWrite, _ := uc.enforcer.Enforce(claims.Email, ns.Name, auth.ObjectConfig, auth.ActionWrite)
		accessible = append(accessible, NamespaceAccess{Name: ns.Name, CanWrite: canWrite})
	}

	sort.Slice(accessible, func(i, j int) bool {
		return accessible[i].Name < accessible[j].Name
	})

	isAdmin, _ := uc.enforcer.Enforce(claims.Email, auth.ObjectAll, auth.ObjectUser, auth.ActionRead)

	var canViewWebhooks bool
	for _, ns := range accessible {
		ok, _ := uc.enforcer.Enforce(claims.Email, ns.Name, auth.ObjectWebhook, auth.ActionRead)
		if ok {
			canViewWebhooks = true

			break
		}
	}

	canManageWebhooks, _ := uc.enforcer.Enforce(claims.Email, auth.ObjectAll, auth.ObjectWebhook, auth.ActionWrite)

	return &MeResult{
		Email:             claims.Email,
		Name:              claims.Name,
		IsAdmin:           isAdmin,
		Namespaces:        accessible,
		CanViewWebhooks:   canViewWebhooks,
		CanManageWebhooks: canManageWebhooks,
	}, nil
}

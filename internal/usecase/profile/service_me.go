package profile

import (
	"context"
	"fmt"
	"sort"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/auth"
)

type NamespaceAccess struct {
	Name     string
	CanWrite bool
}

type MeResult struct {
	Email             string
	Name              string
	IsAdmin           bool
	Namespaces        []NamespaceAccess
	CanViewWebhooks   bool
	CanManageWebhooks bool
}

func (s *Service) Me(ctx context.Context) (*MeResult, error) {
	claims, ok := auth.ClaimsFromContext(ctx)
	if !ok {
		return nil, domain.ErrUnauthorized
	}

	allNamespaces, err := s.ns.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list namespaces: %w", err)
	}

	var accessible []NamespaceAccess
	for _, ns := range allNamespaces {
		canRead, _ := s.enforcer.Enforce(claims.Email, ns.Name, auth.ObjectConfig, auth.ActionRead)
		if !canRead {
			continue
		}
		canWrite, _ := s.enforcer.Enforce(claims.Email, ns.Name, auth.ObjectConfig, auth.ActionWrite)
		accessible = append(accessible, NamespaceAccess{Name: ns.Name, CanWrite: canWrite})
	}

	sort.Slice(accessible, func(i, j int) bool {
		return accessible[i].Name < accessible[j].Name
	})

	isAdmin, _ := s.enforcer.Enforce(claims.Email, auth.ObjectAll, auth.ObjectUser, auth.ActionRead)

	var canViewWebhooks bool
	for _, ns := range accessible {
		ok, _ := s.enforcer.Enforce(claims.Email, ns.Name, auth.ObjectWebhook, auth.ActionRead)
		if ok {
			canViewWebhooks = true

			break
		}
	}

	canManageWebhooks, _ := s.enforcer.Enforce(claims.Email, auth.ObjectAll, auth.ObjectWebhook, auth.ActionWrite)

	return &MeResult{
		Email:             claims.Email,
		Name:              claims.Name,
		IsAdmin:           isAdmin,
		Namespaces:        accessible,
		CanViewWebhooks:   canViewWebhooks,
		CanManageWebhooks: canManageWebhooks,
	}, nil
}

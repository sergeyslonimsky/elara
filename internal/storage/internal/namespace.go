package internal

import (
	"time"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

// NamespaceMeta is the on-disk JSON shape for a domain.Namespace. The bucket
// key is the namespace Name (so Name is NOT serialized in the value).
type NamespaceMeta struct {
	Description string    `json:"description"`
	Locked      bool      `json:"locked,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func DomainToNamespaceMeta(ns *domain.Namespace) NamespaceMeta {
	return NamespaceMeta{
		Description: ns.Description,
		Locked:      ns.Locked,
		CreatedAt:   ns.CreatedAt,
		UpdatedAt:   ns.UpdatedAt,
	}
}

func NamespaceMetaToDomain(m NamespaceMeta, name string) *domain.Namespace {
	return &domain.Namespace{
		Name:        name,
		Description: m.Description,
		Locked:      m.Locked,
		CreatedAt:   m.CreatedAt,
		UpdatedAt:   m.UpdatedAt,
	}
}

package internal_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/sergeyslonimsky/elara/internal/domain"
	storageinternal "github.com/sergeyslonimsky/elara/internal/storage/internal"
)

func TestDomainToNamespaceMeta(t *testing.T) {
	t.Parallel()

	now := time.Now()

	tests := []struct {
		name string
		ns   *domain.Namespace
		want storageinternal.NamespaceMeta
	}{
		{
			name: "full namespace",
			ns: &domain.Namespace{
				Name:        "default",
				Description: "desc",
				Locked:      true,
				CreatedAt:   now,
				UpdatedAt:   now,
			},
			want: storageinternal.NamespaceMeta{
				Description: "desc",
				Locked:      true,
				CreatedAt:   now,
				UpdatedAt:   now,
			},
		},
		{
			name: "zero value namespace",
			ns:   &domain.Namespace{},
			want: storageinternal.NamespaceMeta{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := storageinternal.DomainToNamespaceMeta(tt.ns)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestNamespaceMetaToDomain(t *testing.T) {
	t.Parallel()

	now := time.Now()

	tests := []struct {
		name   string
		meta   storageinternal.NamespaceMeta
		nsName string
		want   *domain.Namespace
	}{
		{
			name: "full meta",
			meta: storageinternal.NamespaceMeta{
				Description: "desc",
				Locked:      true,
				CreatedAt:   now,
				UpdatedAt:   now,
			},
			nsName: "default",
			want: &domain.Namespace{
				Name:        "default",
				Description: "desc",
				Locked:      true,
				CreatedAt:   now,
				UpdatedAt:   now,
			},
		},
		{
			name:   "zero value meta",
			meta:   storageinternal.NamespaceMeta{},
			nsName: "ns2",
			want: &domain.Namespace{
				Name: "ns2",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := storageinternal.NamespaceMetaToDomain(tt.meta, tt.nsName)
			assert.Equal(t, tt.want, got)
		})
	}
}

package filter_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/usecase/filter"
)

func TestService_Catalog(t *testing.T) {
	t.Parallel()

	svc := filter.New(nil, nil, nil, nil)

	got := svc.Catalog()

	require.NotEmpty(t, got)

	want := []filter.CatalogEntry{
		{
			Object: domain.ObjectNamespace,
			Scope:  filter.ScopeNamespace,
			Actions: []domain.Action{
				domain.ActionRead,
				domain.ActionWrite,
				domain.ActionCreate,
				domain.ActionDelete,
				domain.ActionAll,
			},
		},
		{
			Object: domain.ObjectGroup,
			Scope:  filter.ScopeGroup,
			Actions: []domain.Action{
				domain.ActionRead,
				domain.ActionWrite,
				domain.ActionCreate,
				domain.ActionDelete,
				domain.ActionAll,
			},
		},
		{
			Object: domain.ObjectUser,
			Scope:  filter.ScopeGlobal,
			Actions: []domain.Action{
				domain.ActionRead,
				domain.ActionWrite,
				domain.ActionCreate,
				domain.ActionAll,
			},
		},
		{
			Object: domain.ObjectToken,
			Scope:  filter.ScopeGlobal,
			Actions: []domain.Action{
				domain.ActionRead,
				domain.ActionWrite,
				domain.ActionCreate,
				domain.ActionDelete,
				domain.ActionAll,
			},
		},
		{
			Object: domain.ObjectWebhook,
			Scope:  filter.ScopeGlobal,
			Actions: []domain.Action{
				domain.ActionRead,
				domain.ActionWrite,
				domain.ActionCreate,
				domain.ActionDelete,
				domain.ActionAll,
			},
		},
		{
			Object: domain.ObjectClient,
			Scope:  filter.ScopeGlobal,
			Actions: []domain.Action{
				domain.ActionRead,
			},
		},
	}

	assert.Equal(t, want, got)
}

func TestService_Catalog_ReturnsIndependentCopy(t *testing.T) {
	t.Parallel()

	svc := filter.New(nil, nil, nil, nil)

	first := svc.Catalog()
	first[0].Object = domain.ObjectAll

	second := svc.Catalog()

	assert.NotEqual(t, domain.ObjectAll, second[0].Object)
}

func TestLookupCatalogEntry(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		object domain.Object
		want   filter.CatalogEntry
		wantOK bool
	}{
		{
			name:   "known object namespace",
			object: domain.ObjectNamespace,
			want: filter.CatalogEntry{
				Object: domain.ObjectNamespace,
				Scope:  filter.ScopeNamespace,
				Actions: []domain.Action{
					domain.ActionRead,
					domain.ActionWrite,
					domain.ActionCreate,
					domain.ActionDelete,
					domain.ActionAll,
				},
			},
			wantOK: true,
		},
		{
			name:   "known object client is read-only",
			object: domain.ObjectClient,
			want: filter.CatalogEntry{
				Object: domain.ObjectClient,
				Scope:  filter.ScopeGlobal,
				Actions: []domain.Action{
					domain.ActionRead,
				},
			},
			wantOK: true,
		},
		{
			name:   "unassignable object all returns false",
			object: domain.ObjectAll,
			want:   filter.CatalogEntry{},
			wantOK: false,
		},
		{
			name:   "unassignable object policy returns false",
			object: domain.ObjectPolicy,
			want:   filter.CatalogEntry{},
			wantOK: false,
		},
		{
			name:   "unknown object returns false",
			object: domain.Object("does-not-exist"),
			want:   filter.CatalogEntry{},
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, ok := filter.LookupCatalogEntry(tt.object)

			assert.Equal(t, tt.wantOK, ok)
			assert.Equal(t, tt.want, got)
		})
	}
}

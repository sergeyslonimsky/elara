package group

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

func TestValidatePermissionAssignment(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		perm    domain.Permission
		wantErr bool
		wantMsg string
	}{
		{
			name: "namespace_read_on_named_namespace",
			perm: domain.Permission{
				Object: domain.ObjectNamespace,
				Action: domain.ActionRead,
				Domain: domain.NamespaceResource("prod"),
			},
		},
		{
			name: "namespace_write_wildcard",
			perm: domain.Permission{
				Object: domain.ObjectNamespace,
				Action: domain.ActionWrite,
				Domain: domain.DomainAll,
			},
		},
		{
			name: "namespace_object_with_bare_name_rejected",
			perm: domain.Permission{
				Object: domain.ObjectNamespace,
				Action: domain.ActionRead,
				Domain: "prod",
			},
			wantErr: true,
			wantMsg: `must be "*" or "namespace:<name>"`,
		},
		{
			name: "group_read_on_canonical_group_domain",
			perm: domain.Permission{
				Object: domain.ObjectGroup,
				Action: domain.ActionRead,
				Domain: domain.GroupResource("abc-123"),
			},
		},
		{
			name: "group_read_wildcard",
			perm: domain.Permission{
				Object: domain.ObjectGroup,
				Action: domain.ActionRead,
				Domain: domain.DomainAll,
			},
		},
		{
			name: "global_token_read_wildcard",
			perm: domain.Permission{
				Object: domain.ObjectToken,
				Action: domain.ActionRead,
				Domain: domain.DomainAll,
			},
		},
		{
			name: "unknown_object_rejected",
			perm: domain.Permission{
				Object: domain.ObjectPolicy,
				Action: domain.ActionRead,
				Domain: domain.DomainAll,
			},
			wantErr: true,
			wantMsg: "not assignable",
		},
		{
			name: "action_not_in_catalog_for_client",
			perm: domain.Permission{
				Object: domain.ObjectClient,
				Action: domain.ActionDelete,
				Domain: domain.DomainAll,
			},
			wantErr: true,
			wantMsg: "is not allowed",
		},
		{
			name: "global_object_with_namespace_domain_rejected",
			perm: domain.Permission{
				Object: domain.ObjectToken,
				Action: domain.ActionRead,
				Domain: "prod",
			},
			wantErr: true,
			wantMsg: "is global",
		},
		{
			name: "namespace_object_with_empty_domain_rejected",
			perm: domain.Permission{
				Object: domain.ObjectNamespace,
				Action: domain.ActionRead,
				Domain: "",
			},
			wantErr: true,
			wantMsg: "namespace domain",
		},
		{
			name: "group_object_with_empty_domain_rejected",
			perm: domain.Permission{
				Object: domain.ObjectGroup,
				Action: domain.ActionRead,
				Domain: "",
			},
			wantErr: true,
			wantMsg: "group domain",
		},
		{
			name: "group_object_with_bare_id_rejected",
			perm: domain.Permission{
				Object: domain.ObjectGroup,
				Action: domain.ActionRead,
				Domain: "abc-123",
			},
			wantErr: true,
			wantMsg: `must be "*" or "group:<name>"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := validatePermissionAssignment(tt.perm)
			if !tt.wantErr {
				require.NoError(t, err)

				return
			}

			require.Error(t, err)
			var verr *domain.ValidationError
			require.ErrorAs(t, err, &verr)
			require.Contains(t, verr.Error(), tt.wantMsg)
		})
	}
}

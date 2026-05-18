package interceptor

import (
	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/proto/elara/access/v1/accessv1connect"
	"github.com/sergeyslonimsky/elara/internal/proto/elara/clients/v1/clientsv1connect"
	"github.com/sergeyslonimsky/elara/internal/proto/elara/config/v1/configv1connect"
	"github.com/sergeyslonimsky/elara/internal/proto/elara/dashboard/v1/dashboardv1connect"
	"github.com/sergeyslonimsky/elara/internal/proto/elara/group/v1/groupv1connect"
	"github.com/sergeyslonimsky/elara/internal/proto/elara/namespace/v1/namespacev1connect"
	"github.com/sergeyslonimsky/elara/internal/proto/elara/profile/v1/profilev1connect"
	"github.com/sergeyslonimsky/elara/internal/proto/elara/token/v1/tokenv1connect"
	"github.com/sergeyslonimsky/elara/internal/proto/elara/transfer/v1/transferv1connect"
	"github.com/sergeyslonimsky/elara/internal/proto/elara/user/v1/userv1connect"
	"github.com/sergeyslonimsky/elara/internal/proto/elara/webhook/v1/webhookv1connect"
)

// DefaultRBACPolicies maps every globally-enforced procedure to the
// (Object, Action) required to call it. The interceptor checks these against
// the caller's claims at DomainAll (i.e. the caller must have the permission
// globally — namespace-scoped checks live in the usecase).
//
// A new global RPC MUST be added here when proto-registered; otherwise the
// interceptor will fail it closed at runtime with CodePermissionDenied.
func DefaultRBACPolicies() map[string]Permission {
	return map[string]Permission{
		// AccessService — policy / membership management.
		accessv1connect.AccessServiceAssignRoleProcedure:   {Object: domain.ObjectPolicy, Action: domain.ActionWrite},
		accessv1connect.AccessServiceRevokeRoleProcedure:   {Object: domain.ObjectPolicy, Action: domain.ActionWrite},
		accessv1connect.AccessServiceListPoliciesProcedure: {Object: domain.ObjectPolicy, Action: domain.ActionRead},

		// GroupService — user grouping.
		groupv1connect.GroupServiceCreateGroupProcedure: {Object: domain.ObjectGroup, Action: domain.ActionWrite},
		groupv1connect.GroupServiceUpdateGroupProcedure: {Object: domain.ObjectGroup, Action: domain.ActionWrite},
		groupv1connect.GroupServiceDeleteGroupProcedure: {Object: domain.ObjectGroup, Action: domain.ActionWrite},
		groupv1connect.GroupServiceGetGroupProcedure:    {Object: domain.ObjectGroup, Action: domain.ActionRead},
		groupv1connect.GroupServiceListGroupsProcedure:  {Object: domain.ObjectGroup, Action: domain.ActionRead},

		// UserService — user CRUD.
		userv1connect.UserServiceCreateUserProcedure:        {Object: domain.ObjectUser, Action: domain.ActionWrite},
		userv1connect.UserServiceDeleteUserProcedure:        {Object: domain.ObjectUser, Action: domain.ActionWrite},
		userv1connect.UserServiceResetUserPasswordProcedure: {Object: domain.ObjectUser, Action: domain.ActionWrite},
		userv1connect.UserServiceGetUserProcedure:           {Object: domain.ObjectUser, Action: domain.ActionRead},
		userv1connect.UserServiceListUsersProcedure:         {Object: domain.ObjectUser, Action: domain.ActionRead},

		// NamespaceService — global mutation of the namespace registry.
		// Create/Delete affect the registry itself and are admin-only by
		// built-in p-rules. Per-namespace ops (Get/List/Update/Lock/Unlock)
		// are auth-only here — the usecase calls Enforce with the request's
		// namespace, which is what writers/readers in that namespace can
		// resolve through Casbin's recursive role manager.
		namespacev1connect.NamespaceServiceCreateNamespaceProcedure: {
			Object: domain.ObjectNamespace,
			Action: domain.ActionWrite,
		},
		namespacev1connect.NamespaceServiceDeleteNamespaceProcedure: {
			Object: domain.ObjectNamespace,
			Action: domain.ActionWrite,
		},
	}
}

// DefaultRBACAuthOnly returns the set of authenticated procedures that don't
// need a global RBAC check: they're either personal (profile, tokens scoped
// by IssuedBy) or namespace-scoped (config, schema, webhook, clients,
// transfer) and check permissions in the usecase using the namespace from
// the request payload.
func DefaultRBACAuthOnly() map[string]struct{} {
	return map[string]struct{}{
		// Personal — every authenticated user can call.
		profilev1connect.ProfileServiceMeProcedure:             {},
		profilev1connect.ProfileServiceChangePasswordProcedure: {},
		profilev1connect.ProfileServiceLogoutProcedure:         {},

		// TokenService — usecase scopes results to the caller (issuedBy)
		// and elevates for token/read at DomainAll. Whitelist so regular
		// users can manage their own tokens.
		tokenv1connect.TokenServiceCreateTokenProcedure: {},
		tokenv1connect.TokenServiceGetTokenProcedure:    {},
		tokenv1connect.TokenServiceListTokensProcedure:  {},
		tokenv1connect.TokenServiceRevokeTokenProcedure: {},

		// ConfigService — namespace-scoped; usecase calls Enforce with the
		// request's namespace.
		configv1connect.ConfigServiceCreateConfigProcedure:        {},
		configv1connect.ConfigServiceGetConfigProcedure:           {},
		configv1connect.ConfigServiceUpdateConfigProcedure:        {},
		configv1connect.ConfigServiceDeleteConfigProcedure:        {},
		configv1connect.ConfigServiceListConfigsProcedure:         {},
		configv1connect.ConfigServiceGetConfigHistoryProcedure:    {},
		configv1connect.ConfigServiceGetConfigAtRevisionProcedure: {},
		configv1connect.ConfigServiceSearchConfigsProcedure:       {},
		configv1connect.ConfigServiceCopyConfigProcedure:          {},
		configv1connect.ConfigServiceValidateConfigProcedure:      {},
		configv1connect.ConfigServiceWatchConfigsProcedure:        {},
		configv1connect.ConfigServiceGetConfigDiffProcedure:       {},
		configv1connect.ConfigServiceLockConfigProcedure:          {},
		configv1connect.ConfigServiceUnlockConfigProcedure:        {},

		// SchemaService — namespace-scoped.
		configv1connect.SchemaServiceAttachSchemaProcedure:       {},
		configv1connect.SchemaServiceDetachSchemaProcedure:       {},
		configv1connect.SchemaServiceGetSchemaProcedure:          {},
		configv1connect.SchemaServiceGetEffectiveSchemaProcedure: {},
		configv1connect.SchemaServiceListSchemasProcedure:        {},

		// WebhookService — namespace-scoped.
		webhookv1connect.WebhookServiceCreateWebhookProcedure:      {},
		webhookv1connect.WebhookServiceGetWebhookProcedure:         {},
		webhookv1connect.WebhookServiceUpdateWebhookProcedure:      {},
		webhookv1connect.WebhookServiceDeleteWebhookProcedure:      {},
		webhookv1connect.WebhookServiceListWebhooksProcedure:       {},
		webhookv1connect.WebhookServiceGetDeliveryHistoryProcedure: {},

		// ClientsService — namespace-scoped per-client visibility filter.
		clientsv1connect.ClientsServiceListActiveClientsProcedure:         {},
		clientsv1connect.ClientsServiceGetClientProcedure:                 {},
		clientsv1connect.ClientsServiceListHistoricalConnectionsProcedure: {},
		clientsv1connect.ClientsServiceListClientSessionsProcedure:        {},
		clientsv1connect.ClientsServiceWatchClientsProcedure:              {},
		clientsv1connect.ClientsServiceWatchClientProcedure:               {},

		// TransferService — namespace import/export.
		transferv1connect.TransferServiceExportNamespaceProcedure: {},
		transferv1connect.TransferServiceExportAllProcedure:       {},
		transferv1connect.TransferServiceImportNamespaceProcedure: {},

		// NamespaceService — per-namespace ops fall through to usecase
		// where the namespace from the request is used for Enforce.
		namespacev1connect.NamespaceServiceGetNamespaceProcedure:    {},
		namespacev1connect.NamespaceServiceListNamespacesProcedure:  {},
		namespacev1connect.NamespaceServiceUpdateNamespaceProcedure: {},
		namespacev1connect.NamespaceServiceLockNamespaceProcedure:   {},
		namespacev1connect.NamespaceServiceUnlockNamespaceProcedure: {},

		// DashboardService — per-namespace filter happens in the usecase
		// (stats and activity scope themselves to namespaces the caller
		// can read).
		dashboardv1connect.DashboardServiceGetStatsProcedure:     {},
		dashboardv1connect.DashboardServiceListActivityProcedure: {},
	}
}

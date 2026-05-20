import type { AppAbility } from "./ability";

export function uiVisibility(a: AppAbility) {
	return {
		canSeeNamespacesSection: a.can("read", "Namespace"),
		canSeeConfigsSection: a.can("read", "Namespace"),
		canSeeGroupsSection: a.can("read", "Group"),
		canSeeUsersSection: a.can("read", "Group"),
		canSeeTokensSection: a.can("read", "Namespace"),
		canSeeWebhooksSection: a.can("read", "Webhook"),
		// Note: Clients visibility is proxied by Namespace read access as per reasonable approximation (§8)
		canSeeClientsSection: a.can("read", "Namespace"),
		canManageAccess: a.can("write", "all"),
	};
}

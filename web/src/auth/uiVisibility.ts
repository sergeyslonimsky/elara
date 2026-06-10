import type { AppAbility } from "./ability";

export function uiVisibility(a: AppAbility) {
	// Configs/schemas/transfer live under the Namespace object, so their
	// section visibility is the same read check — computed once.
	const canReadNamespace = a.can("read", "Namespace");

	return {
		canSeeNamespacesSection: canReadNamespace,
		canSeeConfigsSection: canReadNamespace,
		canSeeGroupsSection: a.can("read", "Group"),
		canSeeUsersSection: a.can("read", "User"),
		canSeeTokensSection: a.can("read", "Token"),
		canSeeWebhooksSection: a.can("read", "Webhook"),
		canSeeClientsSection: a.can("read", "Client"),
	};
}

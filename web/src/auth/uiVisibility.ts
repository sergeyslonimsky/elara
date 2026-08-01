import type { GetCapabilitiesResponse } from "@/gen/elara/capabilities/v1/capabilities_service_pb";
import type { AppAbility } from "./ability";

export function uiVisibility(
	a: AppAbility,
	capabilities: GetCapabilitiesResponse,
) {
	// Configs/schemas/transfer live under the Namespace object, so their
	// section visibility is the same read check — computed once.
	const canReadNamespace = a.can("read", "Namespace");

	return {
		canSeeNamespacesSection: canReadNamespace,
		canSeeConfigsSection: canReadNamespace,
		canSeeGroupsSection:
			a.can("read", "Group") && capabilities.userManagementEnabled,
		canSeeUsersSection:
			a.can("read", "User") && capabilities.userManagementEnabled,
		canSeeTokensSection:
			a.can("read", "Token") && capabilities.etcdTokenAuthEnabled,
		canSeeWebhooksSection: a.can("read", "Webhook"),
		canSeeClientsSection: a.can("read", "Client"),
	};
}

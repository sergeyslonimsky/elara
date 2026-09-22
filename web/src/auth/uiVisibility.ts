import { AuthType } from "@/gen/elara/auth/v1/auth_pb";
import type { GetCapabilitiesResponse } from "@/gen/elara/capabilities/v1/capabilities_service_pb";
import type { AppAbility } from "./ability";

export function uiVisibility(
	a: AppAbility,
	capabilities: GetCapabilitiesResponse,
	// AuthType.NONE ("passthrough" mode) legitimately has ui.auth.enabled=true
	// with no real identity provider — capabilities.userManagementEnabled is
	// derived from `enabled` alone, so it doesn't know that. The backend
	// rejects CreateUser under AuthType.NONE (internal/handler/v2/user/handler.go),
	// so Users/Groups visibility must also check authType, same as the
	// sidebar's existing `authType !== AuthType.NONE` gate. `undefined` (not
	// yet resolved) is treated as NONE — fail closed.
	authType: AuthType | undefined,
) {
	// Configs/schemas/transfer live under the Namespace object, so their
	// section visibility is the same read check — computed once.
	const canReadNamespace = a.can("read", "Namespace");
	const hasRealAuthProvider =
		authType !== undefined &&
		authType !== AuthType.NONE &&
		authType !== AuthType.UNSPECIFIED;

	return {
		canSeeNamespacesSection: canReadNamespace,
		canSeeConfigsSection: canReadNamespace,
		canSeeGroupsSection:
			a.can("read", "Group") &&
			capabilities.userManagementEnabled &&
			hasRealAuthProvider,
		canSeeUsersSection:
			a.can("read", "User") &&
			capabilities.userManagementEnabled &&
			hasRealAuthProvider,
		canSeeTokensSection:
			a.can("read", "Token") && capabilities.etcdTokenAuthEnabled,
		canSeeWebhooksSection: a.can("read", "Webhook"),
		canSeeClientsSection: a.can("read", "Client"),
	};
}

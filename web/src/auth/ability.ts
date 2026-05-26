import {
	AbilityBuilder,
	createMongoAbility,
	type InferSubjects,
	type MongoAbility,
	subject,
} from "@casl/ability";
import {
	PermissionAction,
	type PermissionAssignment,
	PermissionObject,
} from "@/gen/elara/common/v1/permission_pb";
import type { Group } from "@/gen/elara/group/v1/group_pb";

export type Action = "read" | "write" | "create";

type Tagged<N extends string> = {
	readonly __caslSubjectType__: N;
	domain: string;
};

export type DomainSubjects =
	| Tagged<"Namespace">
	| Tagged<"Config">
	| Tagged<"User">
	| Tagged<"Group">
	| Tagged<"Token">
	| Tagged<"Webhook">;

export type SubjectName =
	| "Namespace"
	| "Config"
	| "User"
	| "Group"
	| "Token"
	| "Webhook";

export type Subject = InferSubjects<DomainSubjects, true> | "all";

export type AppAbility = MongoAbility<[Action, Subject]>;

export function actionOf(action: PermissionAction): Action {
	switch (action) {
		case PermissionAction.READ:
			return "read";
		case PermissionAction.WRITE:
			return "write";
		case PermissionAction.CREATE:
			return "create";
		case PermissionAction.ALL:
			// Invariant: PermissionAction.ALL is handled explicitly in buildAbility
			// to add all three (read/write/create) rules. This branch is defensive.
			return "write";
		default:
			return "read";
	}
}

export function subjectOf(obj: PermissionObject): SubjectName | "all" {
	switch (obj) {
		case PermissionObject.NAMESPACE:
			return "Namespace";
		case PermissionObject.CONFIG:
			return "Config";
		case PermissionObject.USER:
			return "User";
		case PermissionObject.GROUP:
			return "Group";
		case PermissionObject.TOKEN:
			return "Token";
		case PermissionObject.WEBHOOK:
			return "Webhook";
		case PermissionObject.ALL:
			/**
			 * Invariant: backend doesn't currently emit PermissionObject.ALL with a domain.
			 * If it does, CASL will treat it as the 'all' catch-all keyword.
			 */
			return "all";
		default:
			return "all";
	}
}

export function formatAction(action: PermissionAction): string {
	switch (action) {
		case PermissionAction.READ:
			return "READ";
		case PermissionAction.WRITE:
			return "WRITE";
		case PermissionAction.CREATE:
			return "CREATE";
		case PermissionAction.ALL:
			return "ALL";
		default:
			return "UNKNOWN";
	}
}

export function displayObject(obj: PermissionObject): string {
	switch (obj) {
		case PermissionObject.NAMESPACE:
			return "Namespace";
		case PermissionObject.CONFIG:
			return "Config";
		case PermissionObject.USER:
			return "User";
		case PermissionObject.GROUP:
			return "Group";
		case PermissionObject.TOKEN:
			return "Token";
		case PermissionObject.WEBHOOK:
			return "Webhook";
		case PermissionObject.ALL:
			return "All Resources";
		default:
			return "Unknown";
	}
}

export function groupSubject(group: Pick<Group, "name">) {
	return subject("Group", { domain: `group:${group.name}` });
}

export function canManageGroup(ability: AppAbility, group: Group): boolean {
	if (group.isSystem) return false;
	return ability.can("write", groupSubject(group));
}

export function buildAbility(perms: PermissionAssignment[]): AppAbility {
	const { can, build } = new AbilityBuilder<AppAbility>(createMongoAbility);

	if (!perms) {
		return build();
	}

	for (const p of perms) {
		const subjectName = subjectOf(p.object);

		if (
			p.action === PermissionAction.ALL &&
			p.object === PermissionObject.ALL &&
			p.domain === "*"
		) {
			// Superadmin catch-all using CASL 'all' keyword
			can("read", "all");
			can("write", "all");
			can("create", "all");
			continue;
		}

		if (p.domain === "*") {
			if (p.action === PermissionAction.ALL) {
				can("read", subjectName);
				can("write", subjectName);
				can("create", subjectName);
			} else {
				can(actionOf(p.action), subjectName);
			}
		} else {
			if (p.action === PermissionAction.ALL) {
				can("read", subjectName, { domain: p.domain });
				can("write", subjectName, { domain: p.domain });
				can("create", subjectName, { domain: p.domain });
			} else {
				can(actionOf(p.action), subjectName, { domain: p.domain });
			}
		}
	}

	return build();
}

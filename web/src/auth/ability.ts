import {
	AbilityBuilder,
	createMongoAbility,
	type InferSubjects,
	type MongoAbility,
} from "@casl/ability";
import {
	PermissionAction,
	type PermissionAssignment,
	PermissionObject,
} from "@/gen/elara/common/v1/permission_pb";

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

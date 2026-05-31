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

export type Action = "read" | "write" | "create" | "delete";

type Tagged<N extends string> = {
	readonly __caslSubjectType__: N;
	domain: string;
};

export type DomainSubjects =
	| Tagged<"Namespace">
	| Tagged<"Client">
	| Tagged<"User">
	| Tagged<"Group">
	| Tagged<"Token">
	| Tagged<"Webhook">;

export type SubjectName =
	| "Namespace"
	| "Client"
	| "User"
	| "Group"
	| "Token"
	| "Webhook";

export type Subject = InferSubjects<DomainSubjects, true> | "all";

export type AppAbility = MongoAbility<[Action, Subject]>;

// ---- Enum → metadata tables -------------------------------------------------
// Single FE-side source for action/object semantics. `expand` mirrors the
// backend's domain.ActionGrants: a granted action yields the concrete actions
// it satisfies — write ⊇ read (you cannot edit what you cannot read), ALL
// covers everything, create/delete are independent.

interface ActionMeta {
	casl: Action;
	label: string;
	expand: Action[];
}

const ACTION_META: Record<PermissionAction, ActionMeta> = {
	[PermissionAction.UNSPECIFIED]: {
		casl: "read",
		label: "UNKNOWN",
		expand: [],
	},
	[PermissionAction.READ]: { casl: "read", label: "READ", expand: ["read"] },
	[PermissionAction.WRITE]: {
		casl: "write",
		label: "WRITE",
		expand: ["read", "write"],
	},
	[PermissionAction.CREATE]: {
		casl: "create",
		label: "CREATE",
		expand: ["create"],
	},
	[PermissionAction.DELETE]: {
		casl: "delete",
		label: "DELETE",
		expand: ["delete"],
	},
	// Invariant: ALL is expanded to every concrete action; the single `casl`
	// mapping is defensive — callers should use `expand`.
	[PermissionAction.ALL]: {
		casl: "write",
		label: "ALL",
		expand: ["read", "write", "create", "delete"],
	},
};

interface ObjectMeta {
	subject: SubjectName | "all";
	label: string;
}

const OBJECT_META: Record<PermissionObject, ObjectMeta> = {
	[PermissionObject.UNSPECIFIED]: { subject: "all", label: "Unknown" },
	[PermissionObject.NAMESPACE]: { subject: "Namespace", label: "Namespace" },
	[PermissionObject.CLIENT]: { subject: "Client", label: "Client" },
	[PermissionObject.USER]: { subject: "User", label: "User" },
	[PermissionObject.GROUP]: { subject: "Group", label: "Group" },
	[PermissionObject.TOKEN]: { subject: "Token", label: "Token" },
	[PermissionObject.WEBHOOK]: { subject: "Webhook", label: "Webhook" },
	// Invariant: backend doesn't currently emit PermissionObject.ALL with a
	// domain. CASL treats the "all" subject as the superadmin catch-all.
	[PermissionObject.ALL]: { subject: "all", label: "All Resources" },
};

export function actionOf(action: PermissionAction): Action {
	return ACTION_META[action].casl;
}

export function subjectOf(obj: PermissionObject): SubjectName | "all" {
	return OBJECT_META[obj].subject;
}

export function formatAction(action: PermissionAction): string {
	return ACTION_META[action].label;
}

export function displayObject(obj: PermissionObject): string {
	return OBJECT_META[obj].label;
}

// Canonical resource-domain prefixes — kept in sync with internal/domain/rbac.go
// (domain.NamespaceResource / domain.GroupResource). UI code must never inline
// these strings; constructing a subject through groupSubject / namespaceSubject
// is the only sanctioned path.
export const GROUP_DOMAIN_PREFIX = "group:";
export const NAMESPACE_DOMAIN_PREFIX = "namespace:";
export const WILDCARD_DOMAIN = "*";

export function groupResource(id: string): string {
	if (id === WILDCARD_DOMAIN) return WILDCARD_DOMAIN;
	return `${GROUP_DOMAIN_PREFIX}${id}`;
}

export function namespaceResource(name: string): string {
	if (name === WILDCARD_DOMAIN) return WILDCARD_DOMAIN;
	return `${NAMESPACE_DOMAIN_PREFIX}${name}`;
}

export function groupSubject(group: Pick<Group, "id">) {
	return subject("Group", { domain: groupResource(group.id) });
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
		// Global ("*") grants carry no domain condition; scoped grants are gated
		// on { domain }. subjectName "all" is the CASL superadmin catch-all.
		const isGlobal = p.domain === "*";

		for (const action of ACTION_META[p.action].expand) {
			if (isGlobal) {
				can(action, subjectName);
			} else {
				can(action, subjectName, { domain: p.domain });
			}
		}
	}

	return build();
}

// denyAllAbility is a shared, immutable ability that grants nothing. Used as
// the AbilityContext default and the fallback for unauthenticated callers, so
// UI code can treat `ability` as always-present instead of null-checking.
export const denyAllAbility: AppAbility = buildAbility([]);

import { create } from "@bufbuild/protobuf";
import { AbilityBuilder, createMongoAbility, subject } from "@casl/ability";
import { describe, expect, it } from "vitest";
import {
	PermissionAction,
	PermissionAssignmentSchema,
	PermissionObject,
} from "@/gen/elara/common/v1/permission_pb";
import { GroupSchema } from "@/gen/elara/group/v1/group_pb";
import { type AppAbility, buildAbility, canManageGroup } from "./ability";

const pa = (
	object: PermissionObject,
	action: PermissionAction,
	domain: string,
) => create(PermissionAssignmentSchema, { object, action, domain });

describe("buildAbility", () => {
	it("returns empty ability for empty permissions", () => {
		const ability = buildAbility([]);
		expect(ability.can("read", "Namespace")).toBe(false);
	});

	it("handles superadmin wildcard (ALL, ALL, *)", () => {
		const ability = buildAbility([
			pa(PermissionObject.ALL, PermissionAction.ALL, "*"),
		]);
		expect(ability.can("read", "Namespace")).toBe(true);
		expect(ability.can("write", "Group")).toBe(true);
		expect(ability.can("write", "all")).toBe(true);
		expect(ability.can("create", "Namespace")).toBe(true);
		expect(ability.can("create", "all")).toBe(true);
	});

	it("handles explicit (Namespace, CREATE, *) rule", () => {
		const ability = buildAbility([
			pa(PermissionObject.NAMESPACE, PermissionAction.CREATE, "*"),
		]);
		expect(ability.can("create", "Namespace")).toBe(true);
		expect(ability.can("write", "Namespace")).toBe(false);
		expect(ability.can("create", "Group")).toBe(false);
	});

	it("handles explicit (Config, CREATE, prod) rule", () => {
		const ability = buildAbility([
			pa(PermissionObject.CONFIG, PermissionAction.CREATE, "prod"),
		]);
		expect(ability.can("create", subject("Config", { domain: "prod" }))).toBe(
			true,
		);
		expect(ability.can("create", subject("Config", { domain: "dev" }))).toBe(
			false,
		);
		expect(ability.can("write", subject("Config", { domain: "prod" }))).toBe(
			false,
		);
	});

	it("handles global wildcard domain", () => {
		const ability = buildAbility([
			pa(PermissionObject.NAMESPACE, PermissionAction.READ, "*"),
		]);
		expect(ability.can("read", "Namespace")).toBe(true);
		expect(ability.can("read", subject("Namespace", { domain: "any" }))).toBe(
			true,
		);
		expect(ability.can("write", "Namespace")).toBe(false);
	});

	it("handles explicit domain rules", () => {
		const ability = buildAbility([
			pa(PermissionObject.NAMESPACE, PermissionAction.WRITE, "prod"),
		]);
		expect(ability.can("write", subject("Namespace", { domain: "prod" }))).toBe(
			true,
		);
		expect(ability.can("write", subject("Namespace", { domain: "dev" }))).toBe(
			false,
		);
		expect(ability.can("read", subject("Namespace", { domain: "prod" }))).toBe(
			false,
		);
	});

	it("handles PermissionAction.ALL for specific domain", () => {
		const ability = buildAbility([
			pa(PermissionObject.CONFIG, PermissionAction.ALL, "app-1"),
		]);
		expect(ability.can("read", subject("Config", { domain: "app-1" }))).toBe(
			true,
		);
		expect(ability.can("write", subject("Config", { domain: "app-1" }))).toBe(
			true,
		);
		expect(ability.can("create", subject("Config", { domain: "app-1" }))).toBe(
			true,
		);
	});
});

describe("canManageGroup", () => {
	it("returns false when isSystem=true, even with matching write rule", () => {
		const { can, build } = new AbilityBuilder<AppAbility>(createMongoAbility);
		can("write", "Group", { domain: "group:foo" });
		const ability = build();
		const group = create(GroupSchema, {
			id: "g1",
			name: "foo",
			isSystem: true,
		});
		expect(canManageGroup(ability, group)).toBe(false);
	});

	it("returns true when ability has write Group { domain: 'group:foo' } and group is not system", () => {
		const { can, build } = new AbilityBuilder<AppAbility>(createMongoAbility);
		can("write", "Group", { domain: "group:foo" });
		const ability = build();
		const group = create(GroupSchema, {
			id: "g1",
			name: "foo",
			isSystem: false,
		});
		expect(canManageGroup(ability, group)).toBe(true);
	});

	it("returns false when ability lacks the rule", () => {
		const { build } = new AbilityBuilder<AppAbility>(createMongoAbility);
		const ability = build();
		const group = create(GroupSchema, {
			id: "g1",
			name: "foo",
			isSystem: false,
		});
		expect(canManageGroup(ability, group)).toBe(false);
	});
});

import { create } from "@bufbuild/protobuf";
import { subject } from "@casl/ability";
import { describe, expect, it } from "vitest";
import {
	PermissionAction,
	PermissionAssignmentSchema,
	PermissionObject,
} from "@/gen/elara/common/v1/permission_pb";
import { buildAbility } from "./ability";

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
	});
});

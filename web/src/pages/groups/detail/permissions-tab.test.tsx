import { create } from "@bufbuild/protobuf";
import { AbilityBuilder, createMongoAbility } from "@casl/ability";
import { useMutation } from "@connectrpc/connect-query";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, test, vi } from "vitest";
import type { AppAbility } from "@/auth/ability";
import {
	PermissionAction,
	PermissionAssignmentSchema,
	PermissionObject,
} from "@/gen/elara/common/v1/permission_pb";
import { GroupSchema } from "@/gen/elara/group/v1/group_pb";
import { authenticatedContext, TestProviders } from "@/test/test-utils";
import { PermissionsTab } from "./permissions-tab";

vi.mock("@connectrpc/connect-query", async (importOriginal) => {
	const actual = await importOriginal<Record<string, unknown>>();
	return {
		...actual,
		useMutation: vi.fn(() => ({
			mutate: vi.fn(),
			mutateAsync: vi.fn(),
			isPending: false,
		})),
	};
});

vi.mock("@tanstack/react-query", async (importOriginal) => {
	const actual = await importOriginal<Record<string, unknown>>();
	return {
		...actual,
		useQueryClient: vi.fn(() => ({ invalidateQueries: vi.fn() })),
	};
});

vi.mock("sonner", async (importOriginal) => {
	const actual = await importOriginal<Record<string, unknown>>();
	return {
		...actual,
		toast: { success: vi.fn(), error: vi.fn() },
	};
});

const mockGroup = create(GroupSchema, {
	id: "g1",
	name: "developers",
	description: "Dev team",
	isSystem: false,
	metadataVersion: 1n,
	membersVersion: 1n,
	permissionsVersion: 3n,
});

const existingPerm = create(PermissionAssignmentSchema, {
	object: PermissionObject.NAMESPACE,
	action: PermissionAction.READ,
	domain: "prod",
});

function makeAbility(canWrite = true): AppAbility {
	const { can, build } = new AbilityBuilder<AppAbility>(createMongoAbility);
	if (canWrite) {
		can("write", "Group", { domain: "group:developers" });
	}
	return build();
}

describe("PermissionsTab", () => {
	beforeEach(() => {
		vi.clearAllMocks();
	});

	test("renders existing permissions", () => {
		const authContext = authenticatedContext(makeAbility());

		render(
			<TestProviders authContext={authContext}>
				<PermissionsTab group={mockGroup} permissions={[existingPerm]} />
			</TestProviders>,
		);

		// Should show the domain "prod"
		expect(screen.getByText(/prod/)).toBeInTheDocument();
		// Should show action
		expect(screen.getByText(/READ/)).toBeInTheDocument();
	});

	test("shows empty state when no permissions", () => {
		const authContext = authenticatedContext(makeAbility());

		render(
			<TestProviders authContext={authContext}>
				<PermissionsTab group={mockGroup} permissions={[]} />
			</TestProviders>,
		);

		expect(screen.getByText(/no permissions assigned/i)).toBeInTheDocument();
	});

	test("save button disabled when no changes", () => {
		const authContext = authenticatedContext(makeAbility());

		render(
			<TestProviders authContext={authContext}>
				<PermissionsTab group={mockGroup} permissions={[existingPerm]} />
			</TestProviders>,
		);

		expect(
			screen.getByRole("button", { name: /save changes/i }),
		).toBeDisabled();
	});

	test("clicking Remove stages removal and save calls mutation with remove payload", async () => {
		const ue = userEvent.setup();
		const mockMutate = vi.fn();

		vi.mocked(useMutation).mockReturnValue({
			mutate: mockMutate,
			mutateAsync: vi.fn(),
			isPending: false,
		} as unknown as ReturnType<typeof useMutation>);

		const authContext = authenticatedContext(makeAbility());

		render(
			<TestProviders authContext={authContext}>
				<PermissionsTab group={mockGroup} permissions={[existingPerm]} />
			</TestProviders>,
		);

		await ue.click(screen.getByRole("button", { name: /remove/i }));
		await ue.click(screen.getByRole("button", { name: /save changes/i }));

		expect(mockMutate).toHaveBeenCalledWith(
			expect.objectContaining({
				groupId: "g1",
				add: [],
				remove: [existingPerm],
				expectedPermissionsVersion: 3n,
			}),
		);
	});

	test("adding a new permission via form stages it and save includes it in add", async () => {
		const ue = userEvent.setup();
		const mockMutate = vi.fn();

		vi.mocked(useMutation).mockReturnValue({
			mutate: mockMutate,
			mutateAsync: vi.fn(),
			isPending: false,
		} as unknown as ReturnType<typeof useMutation>);

		const authContext = authenticatedContext(makeAbility());

		render(
			<TestProviders authContext={authContext}>
				<PermissionsTab group={mockGroup} permissions={[]} />
			</TestProviders>,
		);

		// Fill in the domain input and click Add
		const domainInput = screen.getByPlaceholderText(/domain/i);
		await ue.type(domainInput, "staging");
		await ue.click(screen.getByRole("button", { name: /^add$/i }));

		// Staged entry should appear (appears in both the list row and the badge summary)
		expect(screen.getAllByText(/staging/).length).toBeGreaterThan(0);

		// Save
		await ue.click(screen.getByRole("button", { name: /save changes/i }));

		expect(mockMutate).toHaveBeenCalledWith(
			expect.objectContaining({
				groupId: "g1",
				add: expect.arrayContaining([
					expect.objectContaining({ domain: "staging" }),
				]),
				remove: [],
				expectedPermissionsVersion: 3n,
			}),
		);
	});

	test("hides add form and remove buttons when user lacks write permission", () => {
		const authContext = authenticatedContext(makeAbility(false));

		render(
			<TestProviders authContext={authContext}>
				<PermissionsTab group={mockGroup} permissions={[existingPerm]} />
			</TestProviders>,
		);

		expect(screen.queryByText(/add permission/i)).not.toBeInTheDocument();
		expect(
			screen.queryByRole("button", { name: /remove/i }),
		).not.toBeInTheDocument();
		expect(
			screen.getByRole("button", { name: /save changes/i }),
		).toBeDisabled();
	});
});

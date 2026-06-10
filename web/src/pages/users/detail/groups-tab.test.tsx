import { create } from "@bufbuild/protobuf";
import { AbilityBuilder, createMongoAbility } from "@casl/ability";
import { useMutation, useQuery } from "@connectrpc/connect-query";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, test, vi } from "vitest";
import type { AppAbility } from "@/auth/ability";
import { GroupSchema } from "@/gen/elara/group/v1/group_pb";
import { UserSchema } from "@/gen/elara/user/v1/user_pb";
import { authenticatedContext, TestProviders } from "@/test/test-utils";
import { GroupsTab } from "./groups-tab";

vi.mock("@connectrpc/connect-query", async (importOriginal) => {
	const actual = await importOriginal<Record<string, unknown>>();
	return {
		...actual,
		useQuery: vi.fn(),
		useMutation: vi.fn(() => ({
			mutate: vi.fn(),
			mutateAsync: vi.fn(),
			isPending: false,
		})),
	};
});

vi.mock("@/lib/queries", () => ({
	invalidateUserGroupGraph: vi.fn(),
}));

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

const mockUser = create(UserSchema, {
	id: "00000000-0000-0000-0000-00000000000a",
	email: "alice@example.com",
	displayName: "Alice",
});

const editableGroup = create(GroupSchema, {
	name: "developers",
	displayName: "Developers Group",
	isSystem: false,
});

const systemGroup = create(GroupSchema, {
	name: "superadmin",
	displayName: "System Administrators",
	isSystem: true,
});

const readOnlyGroup = create(GroupSchema, {
	name: "readonly-group",
	displayName: "Read-only Viewers",
	isSystem: false,
});

function setupAbility(canWriteGroupNames: string[]) {
	const { can, build } = new AbilityBuilder<AppAbility>(createMongoAbility);
	for (const name of canWriteGroupNames) {
		can("write", "Group", { domain: `group:${name}` });
	}
	return build();
}

describe("GroupsTab", () => {
	beforeEach(() => {
		vi.clearAllMocks();
	});

	test("renders group rows with checkboxes", () => {
		const ability = setupAbility(["developers"]);
		const authContext = authenticatedContext(ability);

		vi.mocked(useQuery).mockReturnValue({
			data: { groups: [editableGroup, systemGroup, readOnlyGroup] },
			isLoading: false,
		} as unknown as ReturnType<typeof useQuery>);

		render(
			<TestProviders authContext={authContext}>
				<GroupsTab
					user={mockUser}
					visibleGroupIds={["developers"]}
					membershipVersion={1n}
				/>
			</TestProviders>,
		);

		expect(screen.getByText("Developers Group")).toBeInTheDocument();
		expect(screen.getByText("System Administrators")).toBeInTheDocument();
		expect(screen.getByText("Read-only Viewers")).toBeInTheDocument();
		// READ ONLY badges for system group and non-editable group
		expect(screen.getAllByText("READ ONLY").length).toBeGreaterThanOrEqual(2);
	});

	test("save button is disabled when no changes staged", () => {
		const ability = setupAbility(["developers"]);
		const authContext = authenticatedContext(ability);

		vi.mocked(useQuery).mockReturnValue({
			data: { groups: [editableGroup] },
			isLoading: false,
		} as unknown as ReturnType<typeof useQuery>);

		render(
			<TestProviders authContext={authContext}>
				<GroupsTab
					user={mockUser}
					visibleGroupIds={["developers"]}
					membershipVersion={1n}
				/>
			</TestProviders>,
		);

		expect(
			screen.getByRole("button", { name: /save changes/i }),
		).toBeDisabled();
	});

	test("toggling a checked group stages remove, calls mutation with removeGroupIds", async () => {
		const ue = userEvent.setup();
		const mockMutate = vi.fn();

		const ability = setupAbility(["developers"]);
		const authContext = authenticatedContext(ability);

		vi.mocked(useQuery).mockReturnValue({
			data: { groups: [editableGroup] },
			isLoading: false,
		} as unknown as ReturnType<typeof useQuery>);

		vi.mocked(useMutation).mockReturnValue({
			mutate: mockMutate,
			mutateAsync: vi.fn(),
			isPending: false,
		} as unknown as ReturnType<typeof useMutation>);

		render(
			<TestProviders authContext={authContext}>
				<GroupsTab
					user={mockUser}
					visibleGroupIds={["developers"]}
					membershipVersion={5n}
				/>
			</TestProviders>,
		);

		// Uncheck the group
		const checkbox = screen.getByRole("checkbox", { name: "Developers Group" });
		await ue.click(checkbox);

		await waitFor(() => {
			expect(
				screen.getByRole("button", { name: /save changes/i }),
			).not.toBeDisabled();
		});

		await ue.click(screen.getByRole("button", { name: /save changes/i }));

		expect(mockMutate).toHaveBeenCalledWith(
			expect.objectContaining({
				userId: "00000000-0000-0000-0000-00000000000a",
				removeGroupIds: ["developers"],
				addGroupIds: [],
				expectedVersion: 5n,
			}),
		);
	});

	test("toggling an unchecked editable group stages add", async () => {
		const ue = userEvent.setup();
		const mockMutate = vi.fn();

		const ability = setupAbility(["developers"]);
		const authContext = authenticatedContext(ability);

		vi.mocked(useQuery).mockReturnValue({
			data: { groups: [editableGroup] },
			isLoading: false,
		} as unknown as ReturnType<typeof useQuery>);

		vi.mocked(useMutation).mockReturnValue({
			mutate: mockMutate,
			mutateAsync: vi.fn(),
			isPending: false,
		} as unknown as ReturnType<typeof useMutation>);

		render(
			<TestProviders authContext={authContext}>
				<GroupsTab
					user={mockUser}
					visibleGroupIds={[]}
					membershipVersion={2n}
				/>
			</TestProviders>,
		);

		const checkbox = screen.getByRole("checkbox", { name: "Developers Group" });
		await ue.click(checkbox);
		await ue.click(screen.getByRole("button", { name: /save changes/i }));

		expect(mockMutate).toHaveBeenCalledWith(
			expect.objectContaining({
				addGroupIds: ["developers"],
				removeGroupIds: [],
			}),
		);
	});

	test("system group checkbox is disabled", () => {
		const ability = setupAbility([]);
		const authContext = authenticatedContext(ability);

		vi.mocked(useQuery).mockReturnValue({
			data: { groups: [systemGroup] },
			isLoading: false,
		} as unknown as ReturnType<typeof useQuery>);

		render(
			<TestProviders authContext={authContext}>
				<GroupsTab
					user={mockUser}
					visibleGroupIds={[]}
					membershipVersion={0n}
				/>
			</TestProviders>,
		);

		expect(
			screen.getByRole("checkbox", { name: "System Administrators" }),
		).toHaveAttribute("aria-disabled", "true");
	});

	test("staged add survives a refetch returning the same visibleGroupIds", async () => {
		const ue = userEvent.setup();

		const ability = setupAbility(["developers"]);
		const authContext = authenticatedContext(ability);

		vi.mocked(useQuery).mockReturnValue({
			data: { groups: [editableGroup] },
			isLoading: false,
		} as unknown as ReturnType<typeof useQuery>);

		const { rerender } = render(
			<TestProviders authContext={authContext}>
				<GroupsTab
					user={mockUser}
					visibleGroupIds={[]}
					membershipVersion={3n}
				/>
			</TestProviders>,
		);

		// Stage an add by ticking the editable group's checkbox
		await ue.click(screen.getByRole("checkbox", { name: "Developers Group" }));
		expect(
			screen.getByRole("button", { name: /save changes/i }),
		).not.toBeDisabled();

		// Imitate a background refetch — same server state, new array reference
		rerender(
			<TestProviders authContext={authContext}>
				<GroupsTab
					user={mockUser}
					visibleGroupIds={[]}
					membershipVersion={3n}
				/>
			</TestProviders>,
		);

		// Staged change must persist
		expect(
			screen.getByRole("button", { name: /save changes/i }),
		).not.toBeDisabled();
		expect(
			screen.getByRole("checkbox", { name: "Developers Group" }),
		).toBeChecked();
	});

	test("success mutation triggers toast", async () => {
		const ue = userEvent.setup();
		const { toast } = await import("sonner");

		const ability = setupAbility(["developers"]);
		const authContext = authenticatedContext(ability);

		vi.mocked(useQuery).mockReturnValue({
			data: { groups: [editableGroup] },
			isLoading: false,
		} as unknown as ReturnType<typeof useQuery>);

		let onSuccess: (() => void) | undefined;
		vi.mocked(useMutation).mockImplementation((_method, opts) => {
			onSuccess = opts?.onSuccess as () => void;
			return {
				mutate: vi.fn(() => onSuccess?.()),
				mutateAsync: vi.fn(),
				isPending: false,
			} as unknown as ReturnType<typeof useMutation>;
		});

		render(
			<TestProviders authContext={authContext}>
				<GroupsTab
					user={mockUser}
					visibleGroupIds={[]}
					membershipVersion={0n}
				/>
			</TestProviders>,
		);

		const checkbox = screen.getByRole("checkbox", { name: "Developers Group" });
		await ue.click(checkbox);
		await ue.click(screen.getByRole("button", { name: /save changes/i }));

		expect(toast.success).toHaveBeenCalledWith("Group memberships updated");
	});
});

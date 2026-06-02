import { create } from "@bufbuild/protobuf";
import { AbilityBuilder, createMongoAbility } from "@casl/ability";
import { useMutation, useQuery } from "@connectrpc/connect-query";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, test, vi } from "vitest";
import type { AppAbility } from "@/auth/ability";
import { ItemSchema } from "@/gen/elara/filter/v1/filter_pb";
import { getUsers } from "@/gen/elara/filter/v1/filter_service-FilterService_connectquery";
import { GroupSchema } from "@/gen/elara/group/v1/group_pb";
import { authenticatedContext, TestProviders } from "@/test/test-utils";
import { MembersTab } from "./members-tab";

vi.mock("@connectrpc/connect-query", async (importOriginal) => {
	const actual = await importOriginal<Record<string, unknown>>();
	return {
		...actual,
		useQuery: vi.fn(() => ({ data: { items: [] }, isLoading: false })),
		useMutation: vi.fn(() => ({
			mutate: vi.fn(),
			mutateAsync: vi.fn(),
			isPending: false,
		})),
	};
});

function mockUsersForFilter(
	items: ReturnType<typeof create<typeof ItemSchema>>[],
) {
	vi.mocked(useQuery).mockImplementation(((method: unknown) => {
		if (method === getUsers) {
			return {
				data: { items },
				isLoading: false,
			} as unknown as ReturnType<typeof useQuery>;
		}
		return {
			data: { items: [] },
			isLoading: false,
		} as unknown as ReturnType<typeof useQuery>;
	}) as typeof useQuery);
}

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
	name: "developers",
	displayName: "Developers Group",
	description: "Dev team",
	isSystem: false,
	metadataVersion: 1n,
	membersVersion: 2n,
	permissionsVersion: 1n,
});

function makeAbility(canWrite = true): AppAbility {
	const { can, build } = new AbilityBuilder<AppAbility>(createMongoAbility);
	if (canWrite) {
		can("write", "Group", { domain: "group:developers" });
	}
	return build();
}

describe("MembersTab", () => {
	beforeEach(() => {
		vi.clearAllMocks();
	});

	test("renders visible members count and preselects them in the filter", () => {
		const authContext = authenticatedContext(makeAbility());

		render(
			<TestProviders authContext={authContext}>
				<MembersTab
					group={mockGroup}
					visibleMembers={["alice@example.com", "bob@example.com"]}
				/>
			</TestProviders>,
		);

		expect(screen.getByText(/2 member\(s\) visible/i)).toBeInTheDocument();
		expect(screen.getByRole("combobox")).toHaveTextContent("2 selected");
	});

	test("shows zero count when no visible members", () => {
		const authContext = authenticatedContext(makeAbility());

		render(
			<TestProviders authContext={authContext}>
				<MembersTab group={mockGroup} visibleMembers={[]} />
			</TestProviders>,
		);

		expect(screen.getByText(/0 member\(s\) visible/i)).toBeInTheDocument();
	});

	test("save button disabled when no changes", () => {
		const authContext = authenticatedContext(makeAbility());

		render(
			<TestProviders authContext={authContext}>
				<MembersTab group={mockGroup} visibleMembers={["alice@example.com"]} />
			</TestProviders>,
		);

		expect(
			screen.getByRole("button", { name: /save changes/i }),
		).toBeDisabled();
	});

	test("unchecking a visible member in the filter saves it as removeEmails", async () => {
		const ue = userEvent.setup();
		const mockMutate = vi.fn();

		mockUsersForFilter([
			create(ItemSchema, {
				key: "alice@example.com",
				value: "Alice",
				actions: [],
			}),
		]);
		vi.mocked(useMutation).mockReturnValue({
			mutate: mockMutate,
			mutateAsync: vi.fn(),
			isPending: false,
		} as unknown as ReturnType<typeof useMutation>);

		const authContext = authenticatedContext(makeAbility());

		render(
			<TestProviders authContext={authContext}>
				<MembersTab group={mockGroup} visibleMembers={["alice@example.com"]} />
			</TestProviders>,
		);

		await ue.click(screen.getByRole("combobox"));
		await ue.click(screen.getByRole("option", { name: "Alice" }));

		const saveBtn = screen.getByRole("button", { name: /save changes/i });
		expect(saveBtn).toBeEnabled();
		await ue.click(saveBtn);

		expect(mockMutate).toHaveBeenCalledWith(
			expect.objectContaining({
				groupName: "developers",
				removeEmails: ["alice@example.com"],
				addEmails: [],
				expectedMembersVersion: 2n,
			}),
		);
	});

	test("picking a new user in the filter saves it as addEmails", async () => {
		const ue = userEvent.setup();
		const mockMutate = vi.fn();

		mockUsersForFilter([
			create(ItemSchema, {
				key: "charlie@example.com",
				value: "Charlie",
				actions: [],
			}),
		]);
		vi.mocked(useMutation).mockReturnValue({
			mutate: mockMutate,
			mutateAsync: vi.fn(),
			isPending: false,
		} as unknown as ReturnType<typeof useMutation>);

		const authContext = authenticatedContext(makeAbility());

		render(
			<TestProviders authContext={authContext}>
				<MembersTab group={mockGroup} visibleMembers={[]} />
			</TestProviders>,
		);

		await ue.click(screen.getByRole("combobox"));
		await ue.click(screen.getByText("Charlie"));
		await ue.click(screen.getByRole("button", { name: /save changes/i }));

		expect(mockMutate).toHaveBeenCalledWith(
			expect.objectContaining({
				groupName: "developers",
				addEmails: ["charlie@example.com"],
				removeEmails: [],
				expectedMembersVersion: 2n,
			}),
		);
	});

	test("hides edit controls when user lacks write permission", () => {
		const authContext = authenticatedContext(makeAbility(false));

		render(
			<TestProviders authContext={authContext}>
				<MembersTab group={mockGroup} visibleMembers={["alice@example.com"]} />
			</TestProviders>,
		);

		expect(screen.queryByRole("combobox")).not.toBeInTheDocument();
		expect(
			screen.getByRole("button", { name: /save changes/i }),
		).toBeDisabled();
	});
});

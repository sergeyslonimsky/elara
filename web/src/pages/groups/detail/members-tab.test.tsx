import { create } from "@bufbuild/protobuf";
import { AbilityBuilder, createMongoAbility } from "@casl/ability";
import { useMutation } from "@connectrpc/connect-query";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, test, vi } from "vitest";
import type { AppAbility } from "@/auth/ability";
import { useAuth } from "@/components/auth-provider";
import { GroupSchema } from "@/gen/elara/group/v1/group_pb";
import { TestProviders } from "@/test/test-utils";
import { MembersTab } from "./members-tab";

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

vi.mock("@/components/auth-provider", async (importOriginal) => {
	const actual = await importOriginal<Record<string, unknown>>();
	return {
		...actual,
		useAuth: vi.fn(),
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

	test("renders visible members", () => {
		vi.mocked(useAuth).mockReturnValue({
			state: {
				status: "authenticated",
				ability: makeAbility(),
				authType: 0,
				user: { email: "admin@example.com", name: "Admin" },
			},
			logout: vi.fn(),
		} as unknown as ReturnType<typeof useAuth>);

		render(
			<TestProviders>
				<MembersTab
					group={mockGroup}
					visibleMembers={["alice@example.com", "bob@example.com"]}
				/>
			</TestProviders>,
		);

		expect(screen.getByText("alice@example.com")).toBeInTheDocument();
		expect(screen.getByText("bob@example.com")).toBeInTheDocument();
		expect(screen.getByText(/2 member\(s\) visible/i)).toBeInTheDocument();
	});

	test("shows empty state when no visible members", () => {
		vi.mocked(useAuth).mockReturnValue({
			state: {
				status: "authenticated",
				ability: makeAbility(),
				authType: 0,
				user: { email: "admin@example.com", name: "Admin" },
			},
			logout: vi.fn(),
		} as unknown as ReturnType<typeof useAuth>);

		render(
			<TestProviders>
				<MembersTab group={mockGroup} visibleMembers={[]} />
			</TestProviders>,
		);

		expect(screen.getByText(/no visible members/i)).toBeInTheDocument();
	});

	test("save button disabled when no changes", () => {
		vi.mocked(useAuth).mockReturnValue({
			state: {
				status: "authenticated",
				ability: makeAbility(),
				authType: 0,
				user: { email: "admin@example.com", name: "Admin" },
			},
			logout: vi.fn(),
		} as unknown as ReturnType<typeof useAuth>);

		render(
			<TestProviders>
				<MembersTab group={mockGroup} visibleMembers={["alice@example.com"]} />
			</TestProviders>,
		);

		expect(
			screen.getByRole("button", { name: /save changes/i }),
		).toBeDisabled();
	});

	test("staging a remove and saving calls mutation with removeEmails", async () => {
		const ue = userEvent.setup();
		const mockMutate = vi.fn();

		vi.mocked(useMutation).mockReturnValue({
			mutate: mockMutate,
			mutateAsync: vi.fn(),
			isPending: false,
		} as unknown as ReturnType<typeof useMutation>);

		vi.mocked(useAuth).mockReturnValue({
			state: {
				status: "authenticated",
				ability: makeAbility(),
				authType: 0,
				user: { email: "admin@example.com", name: "Admin" },
			},
			logout: vi.fn(),
		} as unknown as ReturnType<typeof useAuth>);

		render(
			<TestProviders>
				<MembersTab group={mockGroup} visibleMembers={["alice@example.com"]} />
			</TestProviders>,
		);

		// Click the X button on alice's badge to stage removal
		const removeBtn = screen.getByRole("button", {
			name: "Remove alice@example.com",
		});
		await ue.click(removeBtn);

		// Save changes button should now be enabled
		const saveBtn = screen.getByRole("button", { name: /save changes/i });
		expect(saveBtn).toBeEnabled();

		await ue.click(saveBtn);

		expect(mockMutate).toHaveBeenCalledWith(
			expect.objectContaining({
				groupId: "g1",
				removeEmails: ["alice@example.com"],
				addEmails: [],
				expectedMembersVersion: 2n,
			}),
		);
	});

	test("staging an add via input and saving calls mutation with addEmails", async () => {
		const ue = userEvent.setup();
		const mockMutate = vi.fn();

		vi.mocked(useMutation).mockReturnValue({
			mutate: mockMutate,
			mutateAsync: vi.fn(),
			isPending: false,
		} as unknown as ReturnType<typeof useMutation>);

		vi.mocked(useAuth).mockReturnValue({
			state: {
				status: "authenticated",
				ability: makeAbility(),
				authType: 0,
				user: { email: "admin@example.com", name: "Admin" },
			},
			logout: vi.fn(),
		} as unknown as ReturnType<typeof useAuth>);

		render(
			<TestProviders>
				<MembersTab group={mockGroup} visibleMembers={[]} />
			</TestProviders>,
		);

		const input = screen.getByPlaceholderText(/add member email/i);
		await ue.type(input, "charlie@example.com");
		await ue.click(screen.getByRole("button", { name: /^add$/i }));

		// Staged entry should appear
		expect(screen.getByText("+charlie@example.com")).toBeInTheDocument();

		await ue.click(screen.getByRole("button", { name: /save changes/i }));

		expect(mockMutate).toHaveBeenCalledWith(
			expect.objectContaining({
				groupId: "g1",
				addEmails: ["charlie@example.com"],
				removeEmails: [],
				expectedMembersVersion: 2n,
			}),
		);
	});

	test("hides edit controls when user lacks write permission", () => {
		vi.mocked(useAuth).mockReturnValue({
			state: {
				status: "authenticated",
				ability: makeAbility(false),
				authType: 0,
				user: { email: "readonly@example.com", name: "Readonly" },
			},
			logout: vi.fn(),
		} as unknown as ReturnType<typeof useAuth>);

		render(
			<TestProviders>
				<MembersTab group={mockGroup} visibleMembers={["alice@example.com"]} />
			</TestProviders>,
		);

		// No add input visible
		expect(
			screen.queryByPlaceholderText(/add member email/i),
		).not.toBeInTheDocument();
		// No remove buttons on badges
		expect(
			screen.queryByRole("button", { name: "Remove alice@example.com" }),
		).not.toBeInTheDocument();
		// Save button disabled
		expect(
			screen.getByRole("button", { name: /save changes/i }),
		).toBeDisabled();
	});

	test("invalid email shows inline error and does not stage", async () => {
		const ue = userEvent.setup();
		vi.mocked(useAuth).mockReturnValue({
			state: {
				status: "authenticated",
				ability: makeAbility(),
				authType: 0,
				user: { email: "admin@example.com", name: "Admin" },
			},
			logout: vi.fn(),
		} as unknown as ReturnType<typeof useAuth>);

		render(
			<TestProviders>
				<MembersTab group={mockGroup} visibleMembers={[]} />
			</TestProviders>,
		);

		await ue.type(
			screen.getByPlaceholderText(/add member email/i),
			"not-an-email",
		);
		await ue.click(screen.getByRole("button", { name: /^add$/i }));

		expect(screen.getByText(/invalid email/i)).toBeInTheDocument();
		expect(screen.queryByText(/^\+not-an-email$/)).not.toBeInTheDocument();
		expect(
			screen.getByRole("button", { name: /save changes/i }),
		).toBeDisabled();
	});

	test("comma key stages a valid email", async () => {
		const ue = userEvent.setup();
		vi.mocked(useAuth).mockReturnValue({
			state: {
				status: "authenticated",
				ability: makeAbility(),
				authType: 0,
				user: { email: "admin@example.com", name: "Admin" },
			},
			logout: vi.fn(),
		} as unknown as ReturnType<typeof useAuth>);

		render(
			<TestProviders>
				<MembersTab group={mockGroup} visibleMembers={[]} />
			</TestProviders>,
		);

		await ue.type(
			screen.getByPlaceholderText(/add member email/i),
			"charlie@example.com,",
		);

		expect(screen.getByText("+charlie@example.com")).toBeInTheDocument();
	});

	test("duplicate of existing member shows inline error", async () => {
		const ue = userEvent.setup();
		vi.mocked(useAuth).mockReturnValue({
			state: {
				status: "authenticated",
				ability: makeAbility(),
				authType: 0,
				user: { email: "admin@example.com", name: "Admin" },
			},
			logout: vi.fn(),
		} as unknown as ReturnType<typeof useAuth>);

		render(
			<TestProviders>
				<MembersTab group={mockGroup} visibleMembers={["alice@example.com"]} />
			</TestProviders>,
		);

		await ue.type(
			screen.getByPlaceholderText(/add member email/i),
			"alice@example.com",
		);
		await ue.click(screen.getByRole("button", { name: /^add$/i }));

		expect(screen.getByText(/already a member/i)).toBeInTheDocument();
	});
});

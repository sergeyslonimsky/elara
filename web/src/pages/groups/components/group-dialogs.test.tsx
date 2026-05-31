import { create } from "@bufbuild/protobuf";
import { AbilityBuilder, createMongoAbility } from "@casl/ability";
import { useMutation, useQuery } from "@connectrpc/connect-query";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, test, vi } from "vitest";
import type { AppAbility } from "@/auth/ability";
import type { AuthContextType } from "@/components/auth-provider";
import { ItemSchema } from "@/gen/elara/filter/v1/filter_pb";
import { getUsers } from "@/gen/elara/filter/v1/filter_service-FilterService_connectquery";
import { GroupSchema } from "@/gen/elara/group/v1/group_pb";
import { authenticatedContext, TestProviders } from "@/test/test-utils";
import { CreateGroupDialog, DeleteGroupDialog } from "./group-dialogs";

vi.mock("@connectrpc/connect-query", async (importOriginal) => {
	const actual = await importOriginal<Record<string, unknown>>();
	return {
		...actual,
		useQuery: vi.fn(() => ({ data: { groups: [] }, isLoading: false })),
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

function setupAuth(canWriteGroupIds: string[] = []): AuthContextType {
	const { can, build } = new AbilityBuilder<AppAbility>(createMongoAbility);
	for (const id of canWriteGroupIds) {
		can("write", "Group", { domain: `group:${id}` });
	}
	return authenticatedContext(build());
}

vi.mock("sonner", async (importOriginal) => {
	const actual = await importOriginal<Record<string, unknown>>();
	return {
		...actual,
		toast: { success: vi.fn(), error: vi.fn() },
	};
});

const mockGroup = create(GroupSchema, {
	id: "group-1",
	name: "test-group",
	description: "Test group",
	isSystem: false,
	metadataVersion: 1n,
	membersVersion: 1n,
	permissionsVersion: 1n,
});

describe("CreateGroupDialog", () => {
	let authContext: AuthContextType;

	beforeEach(() => {
		vi.clearAllMocks();
		authContext = setupAuth();
		vi.mocked(useQuery).mockReturnValue({
			data: { groups: [] },
			isLoading: false,
		} as unknown as ReturnType<typeof useQuery>);
	});

	test("renders dialog with name field", () => {
		render(
			<TestProviders authContext={authContext}>
				<CreateGroupDialog open={true} onOpenChange={vi.fn()} />
			</TestProviders>,
		);

		expect(
			screen.getByRole("heading", { name: /create group/i }),
		).toBeInTheDocument();
		expect(screen.getByLabelText(/name/i)).toBeInTheDocument();
	});

	test("Create button stays disabled until a valid name is entered", async () => {
		const ue = userEvent.setup();

		render(
			<TestProviders authContext={authContext}>
				<CreateGroupDialog open={true} onOpenChange={vi.fn()} />
			</TestProviders>,
		);

		const createBtn = screen.getByRole("button", { name: /^create$/i });
		expect(createBtn).toBeDisabled();

		// Type something then clear — Zod should report "Name is required"
		await ue.type(screen.getByLabelText(/name/i), "x");
		await ue.clear(screen.getByLabelText(/name/i));

		expect(screen.getByText(/name is required/i)).toBeInTheDocument();
		expect(createBtn).toBeDisabled();

		// Valid name — error clears, button enables
		await ue.type(screen.getByLabelText(/name/i), "developers");
		await waitFor(() => {
			expect(createBtn).not.toBeDisabled();
		});
	});

	test("submitting with members includes them in the payload", async () => {
		const ue = userEvent.setup();
		const mockMutate = vi.fn();

		vi.mocked(useQuery).mockImplementation(((method: unknown) => {
			if (method === getUsers) {
				return {
					data: {
						items: [
							create(ItemSchema, {
								key: "alice@example.com",
								value: "Alice",
								actions: [],
							}),
						],
					},
					isLoading: false,
				} as unknown as ReturnType<typeof useQuery>;
			}
			return {
				data: { groups: [] },
				isLoading: false,
			} as unknown as ReturnType<typeof useQuery>;
		}) as typeof useQuery);
		vi.mocked(useMutation).mockReturnValue({
			mutate: mockMutate,
			mutateAsync: vi.fn(),
			isPending: false,
		} as unknown as ReturnType<typeof useMutation>);

		render(
			<TestProviders authContext={authContext}>
				<CreateGroupDialog open={true} onOpenChange={vi.fn()} />
			</TestProviders>,
		);

		await ue.type(screen.getByLabelText(/^name$/i), "my-group");
		await ue.click(screen.getByRole("combobox"));
		await ue.click(screen.getByText("Alice"));
		await ue.click(screen.getByRole("button", { name: /^create$/i }));

		expect(mockMutate).toHaveBeenCalledWith(
			expect.objectContaining({
				name: "my-group",
				initialMembers: ["alice@example.com"],
			}),
		);
	});

	test("ticking a manager group includes it in initialManagerGroupIds", async () => {
		const ue = userEvent.setup();
		const mockMutate = vi.fn();

		authContext = setupAuth(["mgr-1"]);
		vi.mocked(useQuery).mockReturnValue({
			data: {
				groups: [
					create(GroupSchema, {
						id: "mgr-1",
						name: "developers",
						isSystem: false,
						metadataVersion: 1n,
						membersVersion: 1n,
						permissionsVersion: 1n,
					}),
				],
			},
			isLoading: false,
		} as unknown as ReturnType<typeof useQuery>);
		vi.mocked(useMutation).mockReturnValue({
			mutate: mockMutate,
			mutateAsync: vi.fn(),
			isPending: false,
		} as unknown as ReturnType<typeof useMutation>);

		render(
			<TestProviders authContext={authContext}>
				<CreateGroupDialog open={true} onOpenChange={vi.fn()} />
			</TestProviders>,
		);

		await ue.type(screen.getByLabelText(/^name$/i), "my-group");
		await ue.click(screen.getByRole("checkbox", { name: "developers" }));
		await ue.click(screen.getByRole("button", { name: /^create$/i }));

		expect(mockMutate).toHaveBeenCalledWith(
			expect.objectContaining({
				name: "my-group",
				initialManagerGroupIds: ["mgr-1"],
			}),
		);
	});
});

describe("DeleteGroupDialog", () => {
	beforeEach(() => {
		vi.clearAllMocks();
	});

	test("typing wrong name keeps Delete button disabled", async () => {
		const ue = userEvent.setup();

		render(
			<TestProviders>
				<DeleteGroupDialog
					group={mockGroup}
					open={true}
					onOpenChange={vi.fn()}
				/>
			</TestProviders>,
		);

		await ue.type(screen.getByLabelText(/confirm group name/i), "wrong-name");

		expect(
			screen.getByRole("button", { name: /delete group/i }),
		).toBeDisabled();
	});

	test("typing correct name enables Delete button", async () => {
		const ue = userEvent.setup();

		render(
			<TestProviders>
				<DeleteGroupDialog
					group={mockGroup}
					open={true}
					onOpenChange={vi.fn()}
				/>
			</TestProviders>,
		);

		await ue.type(screen.getByLabelText(/confirm group name/i), "test-group");

		await waitFor(() => {
			expect(
				screen.getByRole("button", { name: /delete group/i }),
			).not.toBeDisabled();
		});
	});

	test("success path invalidates groups", async () => {
		const ue = userEvent.setup();
		const mockInvalidate = vi.fn();

		const { useQueryClient } = await import("@tanstack/react-query");
		vi.mocked(useQueryClient).mockReturnValue({
			invalidateQueries: mockInvalidate,
		} as unknown as ReturnType<typeof useQueryClient>);

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
			<TestProviders>
				<DeleteGroupDialog
					group={mockGroup}
					open={true}
					onOpenChange={vi.fn()}
				/>
			</TestProviders>,
		);

		await ue.type(screen.getByLabelText(/confirm group name/i), "test-group");
		await ue.click(screen.getByRole("button", { name: /delete group/i }));

		expect(mockInvalidate).toHaveBeenCalled();
	});
});

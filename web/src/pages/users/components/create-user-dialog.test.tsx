import { create } from "@bufbuild/protobuf";
import { AbilityBuilder, createMongoAbility } from "@casl/ability";
import { useMutation, useQuery } from "@connectrpc/connect-query";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, test, vi } from "vitest";
import type { AppAbility } from "@/auth/ability";
import { useAuth } from "@/components/auth-provider";
import { AuthType } from "@/gen/elara/auth/v1/auth_pb";
import { GroupSchema } from "@/gen/elara/group/v1/group_pb";
import { TestProviders } from "@/test/test-utils";
import { CreateUserDialog } from "./create-user-dialog";

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

describe("CreateUserDialog", () => {
	beforeEach(() => {
		vi.clearAllMocks();
	});

	test("renders nothing when user lacks create permission", () => {
		vi.mocked(useAuth).mockReturnValue({
			state: {
				status: "authenticated",
				ability: { can: () => false },
				authType: AuthType.NONE,
				user: { email: "admin@example.com", name: "Admin" },
			},
			logout: vi.fn(),
		} as unknown as ReturnType<typeof useAuth>);

		render(
			<TestProviders>
				<CreateUserDialog />
			</TestProviders>,
		);

		expect(
			screen.queryByRole("button", { name: /new user/i }),
		).not.toBeInTheDocument();
	});

	test("renders create button when user has permission", () => {
		vi.mocked(useAuth).mockReturnValue({
			state: {
				status: "authenticated",
				ability: { can: () => true },
				authType: AuthType.NONE,
				user: { email: "admin@example.com", name: "Admin" },
			},
			logout: vi.fn(),
		} as unknown as ReturnType<typeof useAuth>);

		render(
			<TestProviders>
				<CreateUserDialog />
			</TestProviders>,
		);

		expect(
			screen.getByRole("button", { name: /new user/i }),
		).toBeInTheDocument();
	});

	test("opens dialog and submits with basic auth fields", async () => {
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
				ability: { can: () => true },
				authType: AuthType.BASIC,
				user: { email: "admin@example.com", name: "Admin" },
			},
			logout: vi.fn(),
		} as unknown as ReturnType<typeof useAuth>);

		render(
			<TestProviders>
				<CreateUserDialog />
			</TestProviders>,
		);

		await ue.click(screen.getByRole("button", { name: /new user/i }));

		await waitFor(() => {
			expect(
				screen.getByPlaceholderText("user@example.com"),
			).toBeInTheDocument();
		});

		await ue.type(
			screen.getByPlaceholderText("user@example.com"),
			"new@example.com",
		);
		await ue.type(screen.getByPlaceholderText("Jane Doe"), "New User");
		await ue.type(screen.getByPlaceholderText(/at least/i), "password123");

		await ue.click(screen.getByRole("button", { name: /^create$/i }));

		expect(mockMutate).toHaveBeenCalledWith(
			expect.objectContaining({
				email: "new@example.com",
				name: "New User",
				initialPassword: "password123",
			}),
		);
	});

	test("submitting with initialGroupIds includes the selected ids", async () => {
		const ue = userEvent.setup();
		const mockMutate = vi.fn();

		const { can, build } = new AbilityBuilder<AppAbility>(createMongoAbility);
		can("create", "User");
		can("write", "Group", { domain: "group:developers" });
		const ability = build();

		vi.mocked(useAuth).mockReturnValue({
			state: {
				status: "authenticated",
				ability,
				authType: AuthType.BASIC,
				user: { email: "admin@example.com", name: "Admin" },
			},
			logout: vi.fn(),
		} as unknown as ReturnType<typeof useAuth>);

		vi.mocked(useQuery).mockReturnValue({
			data: {
				groups: [
					create(GroupSchema, {
						id: "g1",
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
			<TestProviders>
				<CreateUserDialog />
			</TestProviders>,
		);

		await ue.click(screen.getByRole("button", { name: /new user/i }));

		await waitFor(() => {
			expect(
				screen.getByPlaceholderText("user@example.com"),
			).toBeInTheDocument();
		});

		await ue.type(
			screen.getByPlaceholderText("user@example.com"),
			"new@example.com",
		);
		await ue.type(screen.getByPlaceholderText("Jane Doe"), "New User");
		await ue.type(screen.getByPlaceholderText(/at least/i), "password123");

		await ue.click(screen.getByRole("checkbox", { name: "developers" }));
		await ue.click(screen.getByRole("button", { name: /^create$/i }));

		expect(mockMutate).toHaveBeenCalledWith(
			expect.objectContaining({
				email: "new@example.com",
				initialGroupIds: ["g1"],
			}),
		);
	});
});

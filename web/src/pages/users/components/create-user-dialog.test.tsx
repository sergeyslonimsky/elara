import { create } from "@bufbuild/protobuf";
import { AbilityBuilder, createMongoAbility } from "@casl/ability";
import { useMutation, useQuery } from "@connectrpc/connect-query";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, test, vi } from "vitest";
import { type AppAbility, denyAllAbility } from "@/auth/ability";
import { AuthType } from "@/gen/elara/auth/v1/auth_pb";
import { GroupSchema } from "@/gen/elara/group/v1/group_pb";
import { authenticatedContext, TestProviders } from "@/test/test-utils";
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

function canCreateUserAbility(): AppAbility {
	const { can, build } = new AbilityBuilder<AppAbility>(createMongoAbility);
	can("create", "User");
	return build();
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

describe("CreateUserDialog", () => {
	beforeEach(() => {
		vi.clearAllMocks();
	});

	test("renders nothing when user lacks create permission", () => {
		const authContext = authenticatedContext(denyAllAbility, {
			authType: AuthType.NONE,
		});

		render(
			<TestProviders authContext={authContext}>
				<CreateUserDialog />
			</TestProviders>,
		);

		expect(
			screen.queryByRole("button", { name: /new user/i }),
		).not.toBeInTheDocument();
	});

	test("renders create button when user has permission", () => {
		const authContext = authenticatedContext(canCreateUserAbility(), {
			authType: AuthType.NONE,
		});

		render(
			<TestProviders authContext={authContext}>
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

		const authContext = authenticatedContext(canCreateUserAbility(), {
			authType: AuthType.BASIC,
		});

		render(
			<TestProviders authContext={authContext}>
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
		can("write", "Group", { domain: "group:g1" });
		const ability = build();
		const authContext = authenticatedContext(ability, {
			authType: AuthType.BASIC,
		});

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
			<TestProviders authContext={authContext}>
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

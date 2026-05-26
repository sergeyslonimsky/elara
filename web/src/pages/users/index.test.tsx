import { create } from "@bufbuild/protobuf";
import { useQuery } from "@connectrpc/connect-query";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, test, vi } from "vitest";
import { UserSchema } from "@/gen/elara/user/v1/user_pb";
import { TestProviders } from "@/test/test-utils";
import { UsersPage } from "./index";

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

const mockNavigate = vi.fn();
vi.mock("react-router", async (importOriginal) => {
	const actual = await importOriginal<Record<string, unknown>>();
	return {
		...actual,
		useNavigate: () => mockNavigate,
	};
});

describe("UsersPage", () => {
	beforeEach(() => {
		vi.clearAllMocks();
	});

	test("renders user list", () => {
		vi.mocked(useQuery).mockReturnValue({
			data: {
				users: [
					create(UserSchema, {
						email: "user1@example.com",
						name: "User One",
						provider: "internal",
					}),
					create(UserSchema, {
						email: "user2@example.com",
						name: "User Two",
						provider: "oidc",
					}),
				],
				pagination: { total: 2 },
			},
			isLoading: false,
			error: null,
			refetch: vi.fn(),
			isFetching: false,
		} as unknown as ReturnType<typeof useQuery>);

		render(
			<TestProviders>
				<UsersPage />
			</TestProviders>,
		);

		expect(screen.getByText("user1@example.com")).toBeInTheDocument();
		expect(screen.getByText("User One")).toBeInTheDocument();
		expect(screen.getByText("user2@example.com")).toBeInTheDocument();
		expect(screen.getByText("User Two")).toBeInTheDocument();
		expect(screen.getByText("internal")).toBeInTheDocument();
		expect(screen.getByText("oidc")).toBeInTheDocument();
	});

	test("row click navigates to user detail", async () => {
		const ue = userEvent.setup();
		vi.mocked(useQuery).mockReturnValue({
			data: {
				users: [
					create(UserSchema, {
						email: "user1@example.com",
						name: "User One",
						provider: "internal",
					}),
				],
				pagination: { total: 1 },
			},
			isLoading: false,
			error: null,
			refetch: vi.fn(),
			isFetching: false,
		} as unknown as ReturnType<typeof useQuery>);

		render(
			<TestProviders>
				<UsersPage />
			</TestProviders>,
		);

		await ue.click(screen.getByText("user1@example.com"));
		expect(mockNavigate).toHaveBeenCalledWith("/users/user1%40example.com");
	});

	test("search interaction", async () => {
		const ue = userEvent.setup();
		const refetch = vi.fn();
		vi.mocked(useQuery).mockReturnValue({
			data: { users: [], pagination: { total: 0 } },
			isLoading: false,
			error: null,
			refetch,
			isFetching: false,
		} as unknown as ReturnType<typeof useQuery>);

		render(
			<TestProviders>
				<UsersPage />
			</TestProviders>,
		);

		const searchInput = screen.getByPlaceholderText("Search users...");
		await ue.type(searchInput, "test-query{Enter}");

		expect(useQuery).toHaveBeenCalledWith(
			expect.anything(),
			expect.objectContaining({
				search: "test-query",
			}),
		);
	});

	test("pagination interaction", async () => {
		const ue = userEvent.setup();
		vi.mocked(useQuery).mockReturnValue({
			data: {
				users: [],
				pagination: { total: 50 },
			},
			isLoading: false,
			error: null,
			refetch: vi.fn(),
			isFetching: false,
		} as unknown as ReturnType<typeof useQuery>);

		render(
			<TestProviders>
				<UsersPage />
			</TestProviders>,
		);

		await ue.click(screen.getByRole("button", { name: /next/i }));

		expect(useQuery).toHaveBeenCalledWith(
			expect.anything(),
			expect.objectContaining({
				pagination: { limit: 20, offset: 20 },
			}),
		);
	});

	test("renders skeleton while loading", () => {
		vi.mocked(useQuery).mockReturnValue({
			data: undefined,
			isLoading: true,
			error: null,
			refetch: vi.fn(),
			isFetching: true,
		} as unknown as ReturnType<typeof useQuery>);

		render(
			<TestProviders>
				<UsersPage />
			</TestProviders>,
		);

		// SkeletonList uses skeleton items
		const skeletons = document.querySelectorAll('[data-slot="skeleton"]');
		expect(skeletons.length).toBeGreaterThan(0);
		// Check that table content is NOT there
		expect(screen.queryByRole("table")).not.toBeInTheDocument();
	});

	test("renders error state", () => {
		vi.mocked(useQuery).mockReturnValue({
			data: undefined,
			isLoading: false,
			error: { message: "Failed to fetch" },
			refetch: vi.fn(),
			isFetching: false,
		} as unknown as ReturnType<typeof useQuery>);

		render(
			<TestProviders>
				<UsersPage />
			</TestProviders>,
		);

		expect(screen.getByText("Failed to fetch")).toBeInTheDocument();
	});
});

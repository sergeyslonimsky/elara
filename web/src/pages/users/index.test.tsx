import { useQuery } from "@connectrpc/connect-query";
import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, test, vi } from "vitest";
import { TestProviders } from "@/test/test-utils";
import { UsersPage } from "./index";

vi.mock("@connectrpc/connect-query", async (importOriginal) => {
	const actual = await importOriginal<any>();
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

describe("UsersPage", () => {
	beforeEach(() => {
		vi.clearAllMocks();
	});

	test("renders user list", () => {
		vi.mocked(useQuery).mockReturnValue({
			data: {
				users: [
					{
						email: "user1@example.com",
						name: "User One",
						provider: "internal",
						createdAt: { seconds: 1234567890n, nanos: 0 },
					},
					{
						email: "user2@example.com",
						name: "User Two",
						provider: "oidc",
						createdAt: { seconds: 1234567890n, nanos: 0 },
					},
				],
				pagination: { total: 2 },
			},
			isLoading: false,
			error: null,
			refetch: vi.fn(),
			isFetching: false,
		} as any);

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

	test("search interaction", () => {
		const refetch = vi.fn();
		vi.mocked(useQuery).mockReturnValue({
			data: { users: [], pagination: { total: 0 } },
			isLoading: false,
			error: null,
			refetch,
			isFetching: false,
		} as any);

		render(
			<TestProviders>
				<UsersPage />
			</TestProviders>,
		);

		const searchInput = screen.getByPlaceholderText("Search users...");
		fireEvent.change(searchInput, { target: { value: "test-query" } });

		// Press enter to search
		fireEvent.keyDown(searchInput, { key: "Enter", code: "Enter" });

		expect(useQuery).toHaveBeenCalledWith(
			expect.anything(),
			expect.objectContaining({
				search: "test-query",
			}),
		);
	});

	test("pagination interaction", () => {
		vi.mocked(useQuery).mockReturnValue({
			data: {
				users: [],
				pagination: { total: 50 },
			},
			isLoading: false,
			error: null,
			refetch: vi.fn(),
			isFetching: false,
		} as any);

		render(
			<TestProviders>
				<UsersPage />
			</TestProviders>,
		);

		const nextButton = screen.getByRole("button", { name: /next/i });
		fireEvent.click(nextButton);

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
		} as any);

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
		} as any);

		render(
			<TestProviders>
				<UsersPage />
			</TestProviders>,
		);

		expect(screen.getByText("Failed to fetch")).toBeInTheDocument();
	});
});

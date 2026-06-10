import { create } from "@bufbuild/protobuf";
import { TimestampSchema } from "@bufbuild/protobuf/wkt";
import { useQuery } from "@connectrpc/connect-query";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, test, vi } from "vitest";
import { PermissionAction } from "@/gen/elara/common/v1/permission_pb";
import { TokenSchema } from "@/gen/elara/token/v1/token_pb";
import { TestProviders } from "@/test/test-utils";
import { TokensPage } from "./index";

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

const tokenFixture = create(TokenSchema, {
	id: "tok-1",
	name: "ci-deployer",
	issuedBy: "alice@example.com",
	namespaces: ["production"],
	permission: PermissionAction.WRITE,
	createdAt: create(TimestampSchema, { seconds: 1000n, nanos: 0 }),
});

describe("TokensPage", () => {
	beforeEach(() => {
		vi.clearAllMocks();
	});

	test("renders token list", () => {
		vi.mocked(useQuery).mockReturnValue({
			data: {
				tokens: [tokenFixture],
				pagination: { total: 1 },
			},
			isLoading: false,
			error: null,
			refetch: vi.fn(),
			isFetching: false,
		} as unknown as ReturnType<typeof useQuery>);

		render(
			<TestProviders>
				<TokensPage />
			</TestProviders>,
		);

		expect(screen.getByText("ci-deployer")).toBeInTheDocument();
		expect(screen.getByText("alice@example.com")).toBeInTheDocument();
		expect(screen.getByText("production")).toBeInTheDocument();
	});

	test("search interaction passes queryParams filter", async () => {
		const ue = userEvent.setup();
		vi.mocked(useQuery).mockReturnValue({
			data: { tokens: [], pagination: { total: 0 } },
			isLoading: false,
			error: null,
			refetch: vi.fn(),
			isFetching: false,
		} as unknown as ReturnType<typeof useQuery>);

		render(
			<TestProviders>
				<TokensPage />
			</TestProviders>,
		);

		const searchInput = screen.getByPlaceholderText("Search tokens...");
		await ue.type(searchInput, "deployer{Enter}");

		expect(useQuery).toHaveBeenCalledWith(
			expect.anything(),
			expect.objectContaining({
				filters: expect.objectContaining({
					queryParams: ["deployer"],
				}),
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
				<TokensPage />
			</TestProviders>,
		);

		const skeletons = document.querySelectorAll('[data-slot="skeleton"]');
		expect(skeletons.length).toBeGreaterThan(0);
		expect(screen.queryByRole("table")).not.toBeInTheDocument();
	});

	test("renders error state", () => {
		vi.mocked(useQuery).mockReturnValue({
			data: undefined,
			isLoading: false,
			error: { message: "Failed to fetch tokens" },
			refetch: vi.fn(),
			isFetching: false,
		} as unknown as ReturnType<typeof useQuery>);

		render(
			<TestProviders>
				<TokensPage />
			</TestProviders>,
		);

		expect(screen.getByText("Failed to fetch tokens")).toBeInTheDocument();
	});

	test("renders empty state when no tokens", () => {
		vi.mocked(useQuery).mockReturnValue({
			data: { tokens: [], pagination: { total: 0 } },
			isLoading: false,
			error: null,
			refetch: vi.fn(),
			isFetching: false,
		} as unknown as ReturnType<typeof useQuery>);

		render(
			<TestProviders>
				<TokensPage />
			</TestProviders>,
		);

		expect(screen.getByText("No tokens")).toBeInTheDocument();
	});
});

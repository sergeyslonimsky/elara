import { useQuery } from "@connectrpc/connect-query";
import { render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { getStats } from "@/gen/elara/dashboard/v1/dashboard_service-DashboardService_connectquery";
import { TestProviders } from "@/test/test-utils";
import { DashboardPage } from "./index";

vi.mock("@connectrpc/connect-query", async (importOriginal) => {
	const actual = (await importOriginal()) as unknown as Record<string, unknown>;
	return {
		...actual,
		useQuery: vi.fn(),
	};
});

type UseQueryResult = ReturnType<typeof useQuery>;

describe("DashboardPage", () => {
	const mockUseQuery = vi.mocked(useQuery);

	beforeEach(() => {
		vi.resetAllMocks();
		// Default loading state
		mockUseQuery.mockReturnValue({
			isLoading: true,
			isFetching: false,
			data: undefined,
			error: null,
			refetch: vi.fn(),
		} as unknown as UseQueryResult);
	});

	const renderDashboard = () =>
		render(
			<TestProviders>
				<MemoryRouter>
					<DashboardPage />
				</MemoryRouter>
			</TestProviders>,
		);

	it("renders dashboard shell", () => {
		renderDashboard();
		expect(screen.getByText("Dashboard")).toBeInTheDocument();
	});

	it("renders KPI labels and skeletons when loading", () => {
		const { container } = renderDashboard();
		// "Namespaces" appears in KPI card and in NamespacesCard title
		expect(screen.getAllByText("Namespaces")).toHaveLength(2);
		// "Configs" appears in KPI card and in NamespacesCard table header
		expect(screen.getAllByText("Configs")).toHaveLength(2);
		expect(screen.getByText("Active Clients")).toBeInTheDocument();
		expect(screen.getByText("Global Revision")).toBeInTheDocument();

		// Check for skeletons
		const skeletons = container.querySelectorAll('[data-slot="skeleton"]');
		expect(skeletons.length).toBeGreaterThan(0);
	});

	it("renders data when loaded", () => {
		mockUseQuery.mockImplementation((query) => {
			if (query === getStats) {
				return {
					isLoading: false,
					isFetching: false,
					data: {
						namespaceCount: 10,
						configCount: 50,
						activeClientCount: 5,
						globalRevision: BigInt(100),
					},
					error: null,
					refetch: vi.fn(),
				} as unknown as UseQueryResult;
			}
			return {
				isLoading: false,
				isFetching: false,
				data: undefined,
				error: null,
				refetch: vi.fn(),
			} as unknown as UseQueryResult;
		});

		renderDashboard();

		expect(screen.getByText("10")).toBeInTheDocument();
		expect(screen.getByText("50")).toBeInTheDocument();
		expect(screen.getByText("5")).toBeInTheDocument();
		expect(screen.getByText("100")).toBeInTheDocument();
	});

	it("renders error message on failure", () => {
		mockUseQuery.mockReturnValue({
			isLoading: false,
			isFetching: false,
			error: { message: "API Error" },
			data: undefined,
			refetch: vi.fn(),
		} as unknown as UseQueryResult);

		renderDashboard();
		expect(screen.getAllByText("API Error")).toHaveLength(3);
	});
});

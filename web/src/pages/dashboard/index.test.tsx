import { create } from "@bufbuild/protobuf";
import { useQuery } from "@connectrpc/connect-query";
import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { AuthContextType } from "@/components/auth-provider";
import { AuthType } from "@/gen/elara/auth/v1/auth_pb";
import { MeResponseSchema } from "@/gen/elara/profile/v1/profile_service_pb";
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

	const authContext: AuthContextType = {
		state: {
			status: "authenticated",
			authType: AuthType.NONE,
			user: create(MeResponseSchema, {
				name: "Anonymous",
				email: "anonymous@elara.local",
				isAdmin: true,
				namespaces: [],
				passwordChangeRequired: false,
				picture: "",
				canViewWebhooks: true,
				canManageWebhooks: true,
			}),
		},
		logout: vi.fn(),
	};

	const renderDashboard = () =>
		render(
			<TestProviders authContext={authContext}>
				<DashboardPage />
			</TestProviders>,
		);

	it("renders dashboard shell", () => {
		renderDashboard();
		expect(
			screen.getByRole("heading", { name: "Dashboard" }),
		).toBeInTheDocument();
	});

	it("renders KPI labels and skeletons when loading", () => {
		renderDashboard();
		// "Namespaces" and "Configs" appear in both KpiCards and NamespacesCard
		expect(screen.getAllByText("Namespaces").length).toBeGreaterThanOrEqual(1);
		expect(screen.getAllByText("Configs").length).toBeGreaterThanOrEqual(1);
		expect(screen.getByText("Active Clients")).toBeInTheDocument();
		expect(screen.getByText("Global Revision")).toBeInTheDocument();
		// Skeleton elements rendered by KpiCards (data-slot not data-testid)
		expect(
			document.querySelectorAll('[data-slot="skeleton"]').length,
		).toBeGreaterThan(0);
	});

	it("renders data when loaded", () => {
		mockUseQuery.mockReturnValue({
			isLoading: false,
			isFetching: false,
			data: {
				namespaceCount: 5,
				configCount: 120,
				activeClientCount: 15,
				globalRevision: 0n,
			},
			error: null,
			refetch: vi.fn(),
		} as unknown as UseQueryResult);

		renderDashboard();
		expect(screen.getByText("5")).toBeInTheDocument();
		expect(screen.getByText("120")).toBeInTheDocument();
		expect(screen.getByText("15")).toBeInTheDocument();
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

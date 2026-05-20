import { Ability } from "@casl/ability";
import { useQuery } from "@connectrpc/connect-query";
import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, test, vi } from "vitest";
import { useAuth } from "@/components/auth-provider";
import { TestProviders } from "@/test/test-utils";
import { GroupsPage } from "./index";

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

// Mock useAuth to control permissions
vi.mock("@/components/auth-provider", async (importOriginal) => {
	const actual = await importOriginal<Record<string, unknown>>();
	return {
		...actual,
		useAuth: vi.fn(),
	};
});

describe("GroupsPage", () => {
	beforeEach(() => {
		vi.clearAllMocks();

		// Default auth state: authenticated with write permissions
		vi.mocked(useAuth).mockReturnValue({
			state: {
				status: "authenticated",
				ability: new Ability([{ action: "write", subject: "Group" }]),
				user: { email: "admin@example.com", name: "Admin" },
			},
			logout: vi.fn(),
		} as any);
	});

	test("renders group list", () => {
		vi.mocked(useQuery).mockReturnValue({
			data: {
				groups: [
					{
						id: "group-1",
						name: "Developers",
						members: ["user1@example.com"],
						createdAt: { seconds: 1234567890n, nanos: 0 },
					},
					{
						id: "group-2",
						name: "Admins",
						members: ["admin@example.com", "user2@example.com"],
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
				<GroupsPage />
			</TestProviders>,
		);

		expect(screen.getByText("Developers")).toBeInTheDocument();
		expect(screen.getByText("Admins")).toBeInTheDocument();
		expect(screen.getByText("1 member(s)")).toBeInTheDocument();
		expect(screen.getByText("2 member(s)")).toBeInTheDocument();
	});

	test("pagination interaction", () => {
		vi.mocked(useQuery).mockReturnValue({
			data: { groups: [], pagination: { total: 50 } },
			isLoading: false,
			error: null,
			refetch: vi.fn(),
			isFetching: false,
		} as any);

		render(
			<TestProviders>
				<GroupsPage />
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
});

import { create } from "@bufbuild/protobuf";
import { useQuery } from "@connectrpc/connect-query";
import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, test, vi } from "vitest";
import { UserSchema } from "@/gen/elara/user/v1/user_pb";
import { TestProviders } from "@/test/test-utils";
import { UserDetailPage } from "./index";

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

vi.mock("react-router", async (importOriginal) => {
	const actual = await importOriginal<Record<string, unknown>>();
	return {
		...actual,
		useParams: () => ({ email: "alice%40example.com" }),
		useNavigate: () => vi.fn(),
	};
});

vi.mock("@/components/auth-provider", async (importOriginal) => {
	const actual = await importOriginal<Record<string, unknown>>();
	return {
		...actual,
		useAuth: vi.fn(() => ({
			state: {
				status: "authenticated",
				ability: { can: () => false },
				authType: 0,
				user: { email: "admin@example.com", name: "Admin" },
			},
			logout: vi.fn(),
		})),
	};
});

describe("UserDetailPage", () => {
	beforeEach(() => {
		vi.clearAllMocks();
	});

	test("renders skeleton while loading", () => {
		vi.mocked(useQuery).mockReturnValue({
			data: undefined,
			isLoading: true,
			error: null,
		} as unknown as ReturnType<typeof useQuery>);

		render(
			<TestProviders initialEntries={["/users/alice%40example.com"]}>
				<UserDetailPage />
			</TestProviders>,
		);

		const skeletons = document.querySelectorAll('[data-slot="skeleton"]');
		expect(skeletons.length).toBeGreaterThan(0);
	});

	test("renders error state", () => {
		vi.mocked(useQuery).mockReturnValue({
			data: undefined,
			isLoading: false,
			error: { message: "User not found" },
		} as unknown as ReturnType<typeof useQuery>);

		render(
			<TestProviders initialEntries={["/users/alice%40example.com"]}>
				<UserDetailPage />
			</TestProviders>,
		);

		expect(screen.getByText("User not found")).toBeInTheDocument();
	});

	test("renders user detail when data loaded", () => {
		const user = create(UserSchema, {
			email: "alice@example.com",
			name: "Alice",
			provider: "internal",
		});

		vi.mocked(useQuery).mockReturnValue({
			data: { user, visibleGroupIds: [], membershipVersion: 0n },
			isLoading: false,
			error: null,
		} as unknown as ReturnType<typeof useQuery>);

		render(
			<TestProviders initialEntries={["/users/alice%40example.com"]}>
				<UserDetailPage />
			</TestProviders>,
		);

		expect(screen.getAllByText("Alice").length).toBeGreaterThan(0);
		expect(screen.getAllByText("alice@example.com").length).toBeGreaterThan(0);
	});
});

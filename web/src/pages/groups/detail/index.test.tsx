import { create } from "@bufbuild/protobuf";
import { useQuery } from "@connectrpc/connect-query";
import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, test, vi } from "vitest";
import { denyAllAbility } from "@/auth/ability";
import { GroupSchema } from "@/gen/elara/group/v1/group_pb";
import { authenticatedContext, TestProviders } from "@/test/test-utils";
import { GroupDetailPage } from "./index";

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
		useParams: () => ({ id: "group-1" }),
		useNavigate: () => vi.fn(),
	};
});

vi.mock("@tanstack/react-query", async (importOriginal) => {
	const actual = await importOriginal<Record<string, unknown>>();
	return {
		...actual,
		useQueryClient: vi.fn(() => ({ invalidateQueries: vi.fn() })),
	};
});

describe("GroupDetailPage", () => {
	const authContext = authenticatedContext(denyAllAbility);

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
			<TestProviders
				initialEntries={["/groups/group-1"]}
				authContext={authContext}
			>
				<GroupDetailPage />
			</TestProviders>,
		);

		const skeletons = document.querySelectorAll('[data-slot="skeleton"]');
		expect(skeletons.length).toBeGreaterThan(0);
	});

	test("renders error state", () => {
		vi.mocked(useQuery).mockReturnValue({
			data: undefined,
			isLoading: false,
			error: { message: "Group not found" },
		} as unknown as ReturnType<typeof useQuery>);

		render(
			<TestProviders
				initialEntries={["/groups/group-1"]}
				authContext={authContext}
			>
				<GroupDetailPage />
			</TestProviders>,
		);

		expect(screen.getByText("Group not found")).toBeInTheDocument();
	});

	test("renders group detail when data loaded", () => {
		const group = create(GroupSchema, {
			name: "developers",
			description: "Dev team",
			isSystem: false,
			metadataVersion: 1n,
			membersVersion: 1n,
			permissionsVersion: 1n,
		});

		vi.mocked(useQuery).mockReturnValue({
			data: { group, visibleMembers: [], permissions: [] },
			isLoading: false,
			error: null,
		} as unknown as ReturnType<typeof useQuery>);

		render(
			<TestProviders
				initialEntries={["/groups/group-1"]}
				authContext={authContext}
			>
				<GroupDetailPage />
			</TestProviders>,
		);

		expect(screen.getAllByText("developers").length).toBeGreaterThan(0);
	});
});

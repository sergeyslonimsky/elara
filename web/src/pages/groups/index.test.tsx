import { create } from "@bufbuild/protobuf";
import { Ability } from "@casl/ability";
import { useQuery } from "@connectrpc/connect-query";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, test, vi } from "vitest";
import { useAuth } from "@/components/auth-provider";
import { GroupSchema } from "@/gen/elara/group/v1/group_pb";
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

vi.mock("@/components/auth-provider", async (importOriginal) => {
	const actual = await importOriginal<Record<string, unknown>>();
	return {
		...actual,
		useAuth: vi.fn(),
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

describe("GroupsPage", () => {
	beforeEach(() => {
		vi.clearAllMocks();

		vi.mocked(useAuth).mockReturnValue({
			state: {
				status: "authenticated",
				ability: new Ability([{ action: "write", subject: "Group" }]),
				authType: 0,
				user: { email: "admin@example.com", name: "Admin" },
			},
			logout: vi.fn(),
		} as unknown as ReturnType<typeof useAuth>);
	});

	test("renders group list", () => {
		vi.mocked(useQuery).mockReturnValue({
			data: {
				groups: [
					create(GroupSchema, {
						id: "group-1",
						name: "Developers",
						isSystem: false,
						metadataVersion: 1n,
						membersVersion: 1n,
						permissionsVersion: 1n,
					}),
					create(GroupSchema, {
						id: "group-2",
						name: "Admins",
						isSystem: false,
						metadataVersion: 1n,
						membersVersion: 1n,
						permissionsVersion: 1n,
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
				<GroupsPage />
			</TestProviders>,
		);

		expect(screen.getByText("Developers")).toBeInTheDocument();
		expect(screen.getByText("Admins")).toBeInTheDocument();
	});

	test("row click navigates to group detail", async () => {
		const ue = userEvent.setup();

		vi.mocked(useQuery).mockReturnValue({
			data: {
				groups: [
					create(GroupSchema, {
						id: "group-1",
						name: "Developers",
						isSystem: false,
						metadataVersion: 1n,
						membersVersion: 1n,
						permissionsVersion: 1n,
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
				<GroupsPage />
			</TestProviders>,
		);

		await ue.click(screen.getByText("Developers"));
		expect(mockNavigate).toHaveBeenCalledWith("/groups/group-1");
	});

	test("pagination interaction", async () => {
		const ue = userEvent.setup();
		vi.mocked(useQuery).mockReturnValue({
			data: { groups: [], pagination: { total: 50 } },
			isLoading: false,
			error: null,
			refetch: vi.fn(),
			isFetching: false,
		} as unknown as ReturnType<typeof useQuery>);

		render(
			<TestProviders>
				<GroupsPage />
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
});

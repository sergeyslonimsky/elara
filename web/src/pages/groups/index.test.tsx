import { create } from "@bufbuild/protobuf";
import { Ability } from "@casl/ability";
import { useQuery } from "@connectrpc/connect-query";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, test, vi } from "vitest";
import type { AppAbility } from "@/auth/ability";
import type { AuthContextType } from "@/components/auth-provider";
import { GroupSchema } from "@/gen/elara/group/v1/group_pb";
import { authenticatedContext, TestProviders } from "@/test/test-utils";
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

const mockNavigate = vi.fn();
vi.mock("react-router", async (importOriginal) => {
	const actual = await importOriginal<Record<string, unknown>>();
	return {
		...actual,
		useNavigate: () => mockNavigate,
	};
});

describe("GroupsPage", () => {
	let authContext: AuthContextType;

	beforeEach(() => {
		vi.clearAllMocks();

		authContext = authenticatedContext(
			new Ability([
				{ action: "write", subject: "Group" },
			]) as unknown as AppAbility,
		);
	});

	test("renders group list", () => {
		vi.mocked(useQuery).mockReturnValue({
			data: {
				groups: [
					create(GroupSchema, {
						name: "Developers",
						isSystem: false,
						metadataVersion: 1n,
						membersVersion: 1n,
						permissionsVersion: 1n,
					}),
					create(GroupSchema, {
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
			<TestProviders authContext={authContext}>
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
			<TestProviders authContext={authContext}>
				<GroupsPage />
			</TestProviders>,
		);

		await ue.click(screen.getByText("Developers"));
		expect(mockNavigate).toHaveBeenCalledWith("/groups/Developers");
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
			<TestProviders authContext={authContext}>
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

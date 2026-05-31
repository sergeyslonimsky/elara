import { create } from "@bufbuild/protobuf";
import { AbilityBuilder, createMongoAbility } from "@casl/ability";
import { useMutation } from "@connectrpc/connect-query";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, test, vi } from "vitest";
import type { AppAbility } from "@/auth/ability";
import { GroupSchema } from "@/gen/elara/group/v1/group_pb";
import { authenticatedContext, TestProviders } from "@/test/test-utils";
import { MetadataTab } from "./metadata-tab";

vi.mock("@connectrpc/connect-query", async (importOriginal) => {
	const actual = await importOriginal<Record<string, unknown>>();
	return {
		...actual,
		useMutation: vi.fn(() => ({
			mutate: vi.fn(),
			mutateAsync: vi.fn(),
			isPending: false,
		})),
	};
});

vi.mock("@tanstack/react-query", async (importOriginal) => {
	const actual = await importOriginal<Record<string, unknown>>();
	return {
		...actual,
		useQueryClient: vi.fn(() => ({ invalidateQueries: vi.fn() })),
	};
});

vi.mock("sonner", async (importOriginal) => {
	const actual = await importOriginal<Record<string, unknown>>();
	return {
		...actual,
		toast: { success: vi.fn(), error: vi.fn() },
	};
});

const mockGroup = create(GroupSchema, {
	id: "g1",
	name: "developers",
	description: "Dev team",
	isSystem: false,
	metadataVersion: 1n,
	membersVersion: 1n,
	permissionsVersion: 1n,
});

describe("MetadataTab", () => {
	beforeEach(() => {
		vi.clearAllMocks();
	});

	test("renders form with group name and description", () => {
		const { can, build } = new AbilityBuilder<AppAbility>(createMongoAbility);
		can("write", "Group", { domain: "group:g1" });
		const ability = build();
		const authContext = authenticatedContext(ability);

		render(
			<TestProviders authContext={authContext}>
				<MetadataTab group={mockGroup} />
			</TestProviders>,
		);

		expect(screen.getByLabelText(/name/i)).toHaveValue("developers");
		expect(screen.getByLabelText(/description/i)).toHaveValue("Dev team");
	});

	test("save button disabled when no changes", () => {
		const { can, build } = new AbilityBuilder<AppAbility>(createMongoAbility);
		can("write", "Group", { domain: "group:g1" });
		const ability = build();
		const authContext = authenticatedContext(ability);

		render(
			<TestProviders authContext={authContext}>
				<MetadataTab group={mockGroup} />
			</TestProviders>,
		);

		expect(
			screen.getByRole("button", { name: /save changes/i }),
		).toBeDisabled();
	});

	test("form is disabled for system group", () => {
		const systemGroup = create(GroupSchema, {
			...mockGroup,
			isSystem: true,
		});

		const { can, build } = new AbilityBuilder<AppAbility>(createMongoAbility);
		can("write", "Group", { domain: "group:g1" });
		const ability = build();
		const authContext = authenticatedContext(ability);

		render(
			<TestProviders authContext={authContext}>
				<MetadataTab group={systemGroup} />
			</TestProviders>,
		);

		expect(screen.getByText(/system group/i)).toBeInTheDocument();
		expect(screen.getByLabelText(/name/i)).toBeDisabled();
		expect(screen.getByLabelText(/description/i)).toBeDisabled();
		expect(
			screen.getByRole("button", { name: /save changes/i }),
		).toBeDisabled();
	});

	test("staged edit survives refetch with same metadataVersion", async () => {
		const ue = userEvent.setup();
		const { can, build } = new AbilityBuilder<AppAbility>(createMongoAbility);
		can("write", "Group", { domain: "group:g1" });
		const ability = build();
		const authContext = authenticatedContext(ability);

		const { rerender } = render(
			<TestProviders authContext={authContext}>
				<MetadataTab group={mockGroup} />
			</TestProviders>,
		);

		await ue.clear(screen.getByLabelText(/name/i));
		await ue.type(screen.getByLabelText(/name/i), "edited");
		expect(screen.getByLabelText(/name/i)).toHaveValue("edited");

		// Simulate a background refetch returning a NEW Group object (different
		// JS identity) but the same metadataVersion. The form must NOT reset.
		const sameVersionRefetch = create(GroupSchema, {
			...mockGroup,
			metadataVersion: 1n,
		});
		rerender(
			<TestProviders authContext={authContext}>
				<MetadataTab group={sameVersionRefetch} />
			</TestProviders>,
		);

		expect(screen.getByLabelText(/name/i)).toHaveValue("edited");
	});

	test("server bump of metadataVersion resets the form", async () => {
		const { can, build } = new AbilityBuilder<AppAbility>(createMongoAbility);
		can("write", "Group", { domain: "group:g1" });
		const ability = build();
		const authContext = authenticatedContext(ability);

		const { rerender } = render(
			<TestProviders authContext={authContext}>
				<MetadataTab group={mockGroup} />
			</TestProviders>,
		);

		expect(screen.getByLabelText(/name/i)).toHaveValue("developers");

		// New metadataVersion → form re-syncs to fresh server values
		const bumped = create(GroupSchema, {
			...mockGroup,
			name: "renamed-by-server",
			metadataVersion: 2n,
		});
		rerender(
			<TestProviders authContext={authContext}>
				<MetadataTab group={bumped} />
			</TestProviders>,
		);

		await waitFor(() => {
			expect(screen.getByLabelText(/name/i)).toHaveValue("renamed-by-server");
		});
	});

	test("submits with correct payload", async () => {
		const ue = userEvent.setup();
		const mockMutate = vi.fn();

		vi.mocked(useMutation).mockReturnValue({
			mutate: mockMutate,
			mutateAsync: vi.fn(),
			isPending: false,
		} as unknown as ReturnType<typeof useMutation>);

		const { can, build } = new AbilityBuilder<AppAbility>(createMongoAbility);
		can("write", "Group", { domain: "group:g1" });
		const ability = build();
		const authContext = authenticatedContext(ability);

		render(
			<TestProviders authContext={authContext}>
				<MetadataTab group={mockGroup} />
			</TestProviders>,
		);

		await ue.clear(screen.getByLabelText(/name/i));
		await ue.type(screen.getByLabelText(/name/i), "new-name");

		await ue.click(screen.getByRole("button", { name: /save changes/i }));

		expect(mockMutate).toHaveBeenCalledWith(
			expect.objectContaining({
				id: "g1",
				name: "new-name",
				expectedMetadataVersion: 1n,
			}),
		);
	});
});

import { create } from "@bufbuild/protobuf";
import { AbilityBuilder, createMongoAbility } from "@casl/ability";
import { useMutation, useQuery } from "@connectrpc/connect-query";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, test, vi } from "vitest";
import type { AppAbility } from "@/auth/ability";
import { useAuth } from "@/components/auth-provider";
import { GroupSchema } from "@/gen/elara/group/v1/group_pb";
import { TestProviders } from "@/test/test-utils";
import { EditGroupDialog } from "./group-dialogs";

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

vi.mock("sonner", async (importOriginal) => {
	const actual = await importOriginal<Record<string, unknown>>();
	return {
		...actual,
		toast: {
			success: vi.fn(),
			error: vi.fn(),
		},
	};
});

describe("EditGroupDialog", () => {
	const mockGroup = create(GroupSchema, {
		id: "group-1",
		name: "test-group",
		description: "Initial description",
		members: ["user1@example.com"],
		permissions: [
			{ object: 1, action: 1, domain: "ns1" }, // Namespace Read on ns1
			{ object: 2, action: 2, domain: "ns2" }, // Config Write on ns2
		],
		version: 1n,
	});

	const mockMutation = {
		mutate: vi.fn(),
		isPending: false,
	};

	beforeEach(() => {
		vi.clearAllMocks();
		vi.mocked(useMutation).mockReturnValue(mockMutation as any);
	});

	test("Readonly Passthrough: can edit metadata but not all permissions", async () => {
		// Use real Ability for realistic behavior
		const { can, build } = new AbilityBuilder<AppAbility>(createMongoAbility);

		// Manager rules:
		can("write", "Group", { domain: "group:test-group" });
		can("write", "Namespace", { domain: "ns1" });
		// No permission for ns2

		const ability = build();

		vi.mocked(useAuth).mockReturnValue({
			state: {
				status: "authenticated",
				ability: ability,
				user: { email: "user@example.com", name: "User" },
			},
			logout: vi.fn(),
		} as any);

		vi.mocked(useQuery).mockReturnValue({
			data: { group: mockGroup },
			isLoading: false,
		} as any);

		render(
			<TestProviders>
				<EditGroupDialog group={mockGroup} open={true} onOpenChange={vi.fn()} />
			</TestProviders>,
		);

		await waitFor(() => {
			expect(screen.getByLabelText(/name/i)).toHaveValue("test-group");
		});

		// Metadata should be editable (matches group rule)
		expect(screen.getByLabelText(/name/i)).not.toBeDisabled();

		// Members should be editable (uses group write permission)
		// Check for '×' button on the member badge
		expect(screen.getByText("×")).toBeInTheDocument();

		// Permissions:
		// ns1 permission should be editable (Remove button visible)
		expect(screen.getByText(/Namespace/i)).toBeInTheDocument();
		expect(screen.getByText(/ns1/i)).toBeInTheDocument();
		expect(screen.getByText("Remove")).toBeInTheDocument();

		// ns2 permission should be READ ONLY
		expect(screen.getByText(/Config/i)).toBeInTheDocument();
		expect(screen.getByText(/ns2/i)).toBeInTheDocument();
		expect(screen.getAllByText("READ ONLY")[0]).toBeInTheDocument();

		// Mutation should include BOTH permissions (passthrough)
		fireEvent.change(screen.getByLabelText(/name/i), {
			target: { value: "new-name" },
		});
		fireEvent.click(screen.getByText("Save Changes"));

		expect(mockMutation.mutate).toHaveBeenCalledWith(
			expect.objectContaining({
				name: "new-name",
				permissions: expect.arrayContaining([
					expect.objectContaining({ domain: "ns1" }),
					expect.objectContaining({ domain: "ns2" }),
				]),
			}),
		);
	});

	test("Full readonly: cannot edit metadata", async () => {
		const { build } = new AbilityBuilder<AppAbility>(createMongoAbility);
		const ability = build(); // Empty ability

		vi.mocked(useAuth).mockReturnValue({
			state: {
				status: "authenticated",
				ability: ability,
				user: { email: "user@example.com", name: "User" },
			},
			logout: vi.fn(),
		} as any);

		vi.mocked(useQuery).mockReturnValue({
			data: { group: mockGroup },
			isLoading: false,
		} as any);

		render(
			<TestProviders>
				<EditGroupDialog group={mockGroup} open={true} onOpenChange={vi.fn()} />
			</TestProviders>,
		);

		await waitFor(() => {
			expect(screen.getByLabelText(/name/i)).toHaveValue("test-group");
		});

		// Name input should be disabled
		expect(screen.getByLabelText(/name/i)).toBeDisabled();

		// Save button should be disabled
		expect(screen.getByText("Save Changes")).toBeDisabled();
	});
});

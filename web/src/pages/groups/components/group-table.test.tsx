import { create } from "@bufbuild/protobuf";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, test, vi } from "vitest";
import { GroupSchema } from "@/gen/elara/group/v1/group_pb";
import { TestProviders } from "@/test/test-utils";
import { GroupTable } from "./group-table";

const mockGroup1 = create(GroupSchema, {
	name: "developers",
	isSystem: false,
	metadataVersion: 1n,
	membersVersion: 1n,
	permissionsVersion: 1n,
});

const mockGroup2 = create(GroupSchema, {
	name: "admins",
	isSystem: false,
	metadataVersion: 1n,
	membersVersion: 1n,
	permissionsVersion: 1n,
});

describe("GroupTable", () => {
	test("renders group rows", () => {
		render(
			<TestProviders>
				<GroupTable
					groups={[mockGroup1, mockGroup2]}
					isLoading={false}
					onRowClick={vi.fn()}
					onDelete={vi.fn()}
				/>
			</TestProviders>,
		);

		expect(screen.getByText("developers")).toBeInTheDocument();
		expect(screen.getByText("admins")).toBeInTheDocument();
	});

	test("shows empty state when no groups", () => {
		render(
			<TestProviders>
				<GroupTable
					groups={[]}
					isLoading={false}
					onRowClick={vi.fn()}
					onDelete={vi.fn()}
				/>
			</TestProviders>,
		);

		expect(screen.getByText("No groups")).toBeInTheDocument();
	});

	test("shows no results when query provided", () => {
		render(
			<TestProviders>
				<GroupTable
					groups={[]}
					isLoading={false}
					query="foo"
					onRowClick={vi.fn()}
					onDelete={vi.fn()}
				/>
			</TestProviders>,
		);

		expect(screen.getByText("No groups found")).toBeInTheDocument();
	});

	test("calls onRowClick when row is clicked", async () => {
		const ue = userEvent.setup();
		const handleClick = vi.fn();

		render(
			<TestProviders>
				<GroupTable
					groups={[mockGroup1]}
					isLoading={false}
					onRowClick={handleClick}
					onDelete={vi.fn()}
				/>
			</TestProviders>,
		);

		await ue.click(screen.getByText("developers"));
		expect(handleClick).toHaveBeenCalledWith(
			expect.objectContaining({ name: "developers" }),
			expect.anything(),
		);
	});

	test("calls onDelete when delete button clicked", async () => {
		const ue = userEvent.setup();
		const handleDelete = vi.fn();

		render(
			<TestProviders>
				<GroupTable
					groups={[mockGroup1]}
					isLoading={false}
					onRowClick={vi.fn()}
					onDelete={handleDelete}
				/>
			</TestProviders>,
		);

		await ue.click(screen.getByTitle("Delete Group"));
		expect(handleDelete).toHaveBeenCalledWith(
			expect.objectContaining({ name: "developers" }),
		);
	});
});

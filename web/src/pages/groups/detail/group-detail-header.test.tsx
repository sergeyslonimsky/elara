import { create } from "@bufbuild/protobuf";
import { render, screen } from "@testing-library/react";
import { describe, expect, test } from "vitest";
import { GroupSchema } from "@/gen/elara/group/v1/group_pb";
import { TestProviders } from "@/test/test-utils";
import { GroupDetailHeader } from "./group-detail-header";

describe("GroupDetailHeader", () => {
	test("renders group name and description", () => {
		const group = create(GroupSchema, {
			name: "developers",
			description: "Dev team",
			isSystem: false,
			metadataVersion: 1n,
			membersVersion: 1n,
			permissionsVersion: 1n,
		});

		render(
			<TestProviders>
				<GroupDetailHeader group={group} />
			</TestProviders>,
		);

		expect(
			screen.getByRole("heading", { name: "developers" }),
		).toBeInTheDocument();
		expect(screen.getByText("Dev team")).toBeInTheDocument();
	});

	test("shows System badge for system group", () => {
		const group = create(GroupSchema, {
			name: "superadmin",
			isSystem: true,
			metadataVersion: 1n,
			membersVersion: 1n,
			permissionsVersion: 1n,
		});

		render(
			<TestProviders>
				<GroupDetailHeader group={group} />
			</TestProviders>,
		);

		expect(screen.getAllByText("System").length).toBeGreaterThan(0);
	});
});

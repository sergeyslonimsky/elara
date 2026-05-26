import { create } from "@bufbuild/protobuf";
import { render, screen } from "@testing-library/react";
import { describe, expect, test } from "vitest";
import { GroupSchema } from "@/gen/elara/group/v1/group_pb";
import { TestProviders } from "@/test/test-utils";
import { GroupDetailHeader } from "./group-detail-header";

describe("GroupDetailHeader", () => {
	test("renders group name and id", () => {
		const group = create(GroupSchema, {
			id: "g1",
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

		expect(screen.getByText("developers")).toBeInTheDocument();
		expect(screen.getByText("Dev team")).toBeInTheDocument();
		expect(screen.getByText("g1")).toBeInTheDocument();
	});

	test("shows System badge for system group", () => {
		const group = create(GroupSchema, {
			id: "g2",
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

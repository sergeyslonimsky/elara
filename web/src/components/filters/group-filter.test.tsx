import { create } from "@bufbuild/protobuf";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { PermissionAction } from "@/gen/elara/common/v1/permission_pb";
import { ItemSchema } from "@/gen/elara/filter/v1/filter_pb";
import { getGroups } from "@/gen/elara/filter/v1/filter_service-FilterService_connectquery";
import { GroupFilter } from "./group-filter";

const mockUseQuery = vi.fn();
vi.mock("@connectrpc/connect-query", () => ({
	useQuery: (...args: unknown[]) => mockUseQuery(...args),
}));

describe("GroupFilter", () => {
	it("calls getGroups with empty actions by default", () => {
		mockUseQuery.mockReturnValue({ data: { items: [] }, isLoading: false });

		render(<GroupFilter value={[]} onValueChange={() => {}} />);

		expect(mockUseQuery).toHaveBeenCalledWith(getGroups, {
			filters: { query: "" },
			actions: [],
		});
	});

	it("forwards actions into the request", () => {
		mockUseQuery.mockReturnValue({ data: { items: [] }, isLoading: false });

		render(
			<GroupFilter
				value={[]}
				onValueChange={() => {}}
				permissionActions={[PermissionAction.READ]}
			/>,
		);

		expect(mockUseQuery).toHaveBeenCalledWith(getGroups, {
			filters: { query: "" },
			actions: [PermissionAction.READ],
		});
	});

	it("renders items and reports selection", async () => {
		const items = [
			create(ItemSchema, { key: "admins", value: "admins", actions: [] }),
			create(ItemSchema, { key: "devs", value: "developers", actions: [] }),
		];
		mockUseQuery.mockReturnValue({ data: { items }, isLoading: false });

		const onValueChange = vi.fn();
		const user = userEvent.setup();

		render(<GroupFilter value={[]} onValueChange={onValueChange} />);

		await user.click(screen.getByRole("combobox"));
		await user.click(screen.getByText("developers"));

		expect(onValueChange).toHaveBeenCalledWith(["devs"]);
	});
});

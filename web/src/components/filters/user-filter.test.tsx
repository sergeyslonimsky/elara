import { create } from "@bufbuild/protobuf";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { PermissionAction } from "@/gen/elara/common/v1/permission_pb";
import { ItemSchema } from "@/gen/elara/filter/v1/filter_pb";
import { getUsers } from "@/gen/elara/filter/v1/filter_service-FilterService_connectquery";
import { UserFilter } from "./user-filter";

const mockUseQuery = vi.fn();
vi.mock("@connectrpc/connect-query", () => ({
	useQuery: (...args: unknown[]) => mockUseQuery(...args),
}));

describe("UserFilter", () => {
	it("calls getUsers with empty actions by default", () => {
		mockUseQuery.mockReturnValue({ data: { items: [] }, isLoading: false });

		render(<UserFilter value={[]} onValueChange={() => {}} />);

		expect(mockUseQuery).toHaveBeenCalledWith(getUsers, {
			filters: { query: "" },
			actions: [],
		});
	});

	it("forwards actions into the request", () => {
		mockUseQuery.mockReturnValue({ data: { items: [] }, isLoading: false });

		render(
			<UserFilter
				value={[]}
				onValueChange={() => {}}
				permissionActions={[PermissionAction.WRITE]}
			/>,
		);

		expect(mockUseQuery).toHaveBeenCalledWith(getUsers, {
			filters: { query: "" },
			actions: [PermissionAction.WRITE],
		});
	});

	it("renders items and reports selection", async () => {
		const items = [
			create(ItemSchema, { key: "alice", value: "Alice", actions: [] }),
			create(ItemSchema, { key: "bob", value: "Bob", actions: [] }),
		];
		mockUseQuery.mockReturnValue({ data: { items }, isLoading: false });

		const onValueChange = vi.fn();
		const user = userEvent.setup();

		render(<UserFilter value={[]} onValueChange={onValueChange} />);

		await user.click(screen.getByRole("combobox"));
		await user.click(screen.getByText("Bob"));

		expect(onValueChange).toHaveBeenCalledWith(["bob"]);
	});
});

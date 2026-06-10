import { create } from "@bufbuild/protobuf";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { PermissionAction } from "@/gen/elara/common/v1/permission_pb";
import { ItemSchema } from "@/gen/elara/filter/v1/filter_pb";
import { getNamespaces } from "@/gen/elara/filter/v1/filter_service-FilterService_connectquery";
import { NamespaceFilter } from "./namespace-filter";

const mockUseQuery = vi.fn();
vi.mock("@connectrpc/connect-query", () => ({
	useQuery: (...args: unknown[]) => mockUseQuery(...args),
}));

describe("NamespaceFilter", () => {
	it("calls getNamespaces with empty actions by default", () => {
		mockUseQuery.mockReturnValue({ data: { items: [] }, isLoading: false });

		render(<NamespaceFilter value={[]} onValueChange={() => {}} />);

		expect(mockUseQuery).toHaveBeenCalledWith(getNamespaces, {
			filters: { query: "" },
			actions: [],
		});
	});

	it("forwards actions into the request", () => {
		mockUseQuery.mockReturnValue({ data: { items: [] }, isLoading: false });

		render(
			<NamespaceFilter
				value={[]}
				onValueChange={() => {}}
				permissionActions={[PermissionAction.WRITE]}
			/>,
		);

		expect(mockUseQuery).toHaveBeenCalledWith(getNamespaces, {
			filters: { query: "" },
			actions: [PermissionAction.WRITE],
		});
	});

	it("renders items from the response and reports selection", async () => {
		const items = [
			create(ItemSchema, { key: "default", value: "default", actions: [] }),
			create(ItemSchema, { key: "prod", value: "production", actions: [] }),
		];
		mockUseQuery.mockReturnValue({ data: { items }, isLoading: false });

		const onValueChange = vi.fn();
		const user = userEvent.setup();

		render(<NamespaceFilter value={[]} onValueChange={onValueChange} />);

		await user.click(screen.getByRole("combobox"));
		await user.click(screen.getByText("production"));

		expect(onValueChange).toHaveBeenCalledWith(["prod"]);
	});
});

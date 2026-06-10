import { create } from "@bufbuild/protobuf";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState } from "react";
import { describe, expect, it, vi } from "vitest";
import { ItemSchema } from "@/gen/elara/filter/v1/filter_pb";
import { FilterCombobox, type FilterComboboxProps } from "./filter-combobox";

function ControlledSearch({
	onSearchChange,
	...rest
}: Omit<FilterComboboxProps, "searchValue">) {
	const [search, setSearch] = useState("");
	return (
		<FilterCombobox
			{...rest}
			searchValue={search}
			onSearchChange={(v) => {
				setSearch(v);
				onSearchChange(v);
			}}
		/>
	);
}

const items = [
	create(ItemSchema, { key: "default", value: "default", actions: [] }),
	create(ItemSchema, { key: "prod", value: "production", actions: [] }),
	create(ItemSchema, { key: "stg", value: "staging", actions: [] }),
];

const noop = () => {};

describe("FilterCombobox", () => {
	it("shows placeholder when nothing is selected", () => {
		render(
			<FilterCombobox
				items={items}
				value={[]}
				onValueChange={noop}
				searchValue=""
				onSearchChange={noop}
				placeholder="Select namespaces..."
			/>,
		);

		expect(screen.getByText("Select namespaces...")).toBeInTheDocument();
	});

	it("shows the item value when exactly one is selected", () => {
		render(
			<FilterCombobox
				items={items}
				value={["prod"]}
				onValueChange={noop}
				searchValue=""
				onSearchChange={noop}
				placeholder="Select namespaces..."
			/>,
		);

		expect(screen.getByText("production")).toBeInTheDocument();
		expect(screen.queryByText("Select namespaces...")).not.toBeInTheDocument();
	});

	it('shows "N selected" when multiple items are selected', () => {
		render(
			<FilterCombobox
				items={items}
				value={["prod", "stg"]}
				onValueChange={noop}
				searchValue=""
				onSearchChange={noop}
				placeholder="Select namespaces..."
			/>,
		);

		expect(screen.getByText("2 selected")).toBeInTheDocument();
	});

	it("calls onValueChange when an item is picked", async () => {
		const onValueChange = vi.fn();
		const user = userEvent.setup();

		render(
			<FilterCombobox
				items={items}
				value={[]}
				onValueChange={onValueChange}
				searchValue=""
				onSearchChange={noop}
				placeholder="Select..."
			/>,
		);

		await user.click(screen.getByRole("combobox"));
		await user.click(screen.getByText("production"));

		expect(onValueChange).toHaveBeenCalledWith(["prod"]);
	});

	it("forwards the input value to onSearchChange", async () => {
		const onSearchChange = vi.fn();
		const user = userEvent.setup();

		render(
			<ControlledSearch
				items={items}
				value={[]}
				onValueChange={noop}
				onSearchChange={onSearchChange}
				placeholder="Select..."
				searchPlaceholder="Search namespaces..."
			/>,
		);

		await user.click(screen.getByRole("combobox"));
		await user.type(screen.getByPlaceholderText("Search namespaces..."), "pr");

		expect(onSearchChange.mock.calls.at(-1)?.[0]).toBe("pr");
	});

	it("emits a single string in single-select mode", async () => {
		const onValueChange = vi.fn();
		const user = userEvent.setup();

		render(
			<FilterCombobox
				items={items}
				multiple={false}
				value={[]}
				onValueChange={onValueChange}
				searchValue=""
				onSearchChange={noop}
				placeholder="Select..."
			/>,
		);

		await user.click(screen.getByRole("combobox"));
		await user.click(screen.getByText("staging"));

		expect(onValueChange).toHaveBeenCalledWith(["stg"]);
	});
});

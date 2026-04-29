import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { DataTable } from "./data-table";

describe("DataTable", () => {
	const columns = [
		{
			accessorKey: "name",
			header: "Name",
		},
		{
			accessorKey: "age",
			header: "Age",
		},
	];

	const data = [
		{ id: 1, name: "Alice", age: 30 },
		{ id: 2, name: "Bob", age: 25 },
	];

	it("renders headers and data correctly", () => {
		render(<DataTable columns={columns} data={data} />);

		expect(screen.getByText("Name")).toBeInTheDocument();
		expect(screen.getByText("Age")).toBeInTheDocument();
		expect(screen.getByText("Alice")).toBeInTheDocument();
		expect(screen.getByText("Bob")).toBeInTheDocument();
	});

	it("calls onRowClick when a row is clicked", async () => {
		const onRowClick = vi.fn();
		const user = userEvent.setup();
		render(<DataTable columns={columns} data={data} onRowClick={onRowClick} />);

		const row = screen.getByRole("row", { name: /Alice/i });
		await user.click(row);
		expect(onRowClick).toHaveBeenCalledWith(data[0], expect.anything());
	});

	it("handles keyboard navigation for row click", async () => {
		const onRowClick = vi.fn();
		const user = userEvent.setup();
		render(<DataTable columns={columns} data={data} onRowClick={onRowClick} />);

		const row = screen.getByRole("row", { name: /Bob/i });
		row.focus();
		await user.keyboard("{Enter}");
		expect(onRowClick).toHaveBeenCalledWith(data[1], expect.anything());
	});
});

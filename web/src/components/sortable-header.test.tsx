import type { Column } from "@tanstack/react-table";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { SortableHeader } from "./sortable-header";

describe("SortableHeader", () => {
	const mockColumn = {
		getIsSorted: vi.fn(),
		toggleSorting: vi.fn(),
	} as unknown as Column<unknown>;

	beforeEach(() => {
		vi.clearAllMocks();
	});

	it("renders correctly and calls toggleSorting on click", async () => {
		const user = userEvent.setup();
		vi.mocked(mockColumn.getIsSorted).mockReturnValue(false);

		render(<SortableHeader column={mockColumn}>Name</SortableHeader>);

		const button = screen.getByRole("button", { name: /name/i });
		expect(button).toBeInTheDocument();

		await user.click(button);
		expect(mockColumn.toggleSorting).toHaveBeenCalledWith(false);
	});

	it("shows up arrow when sorted asc", () => {
		vi.mocked(mockColumn.getIsSorted).mockReturnValue("asc");
		render(<SortableHeader column={mockColumn}>Name</SortableHeader>);
		expect(
			screen.getByRole("button").querySelector(".lucide-arrow-up"),
		).toBeInTheDocument();
	});

	it("shows down arrow when sorted desc", () => {
		vi.mocked(mockColumn.getIsSorted).mockReturnValue("desc");
		render(<SortableHeader column={mockColumn}>Name</SortableHeader>);
		expect(
			screen.getByRole("button").querySelector(".lucide-arrow-down"),
		).toBeInTheDocument();
	});
});

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { PaginationControls } from "./pagination-controls";

describe("PaginationControls", () => {
	it("renders correctly", () => {
		render(
			<PaginationControls
				total={100}
				pageSize={10}
				offset={0}
				onOffsetChange={() => {}}
				onPageSizeChange={() => {}}
			/>,
		);

		expect(screen.getByText("1–10 of 100")).toBeInTheDocument();
		expect(screen.getByRole("button", { name: "Page 1" })).toBeInTheDocument();
		expect(screen.getByRole("button", { name: "Page 10" })).toBeInTheDocument();
	});

	it("calls onOffsetChange when next/prev clicked", async () => {
		const onOffsetChange = vi.fn();
		const user = userEvent.setup();
		render(
			<PaginationControls
				total={100}
				pageSize={10}
				offset={10}
				onOffsetChange={onOffsetChange}
				onPageSizeChange={() => {}}
			/>,
		);

		await user.click(screen.getByRole("button", { name: "Next page" }));
		expect(onOffsetChange).toHaveBeenCalledWith(20);

		await user.click(screen.getByRole("button", { name: "Previous page" }));
		expect(onOffsetChange).toHaveBeenCalledWith(0);
	});

	it("calls onOffsetChange when a page number is clicked", async () => {
		const onOffsetChange = vi.fn();
		const user = userEvent.setup();
		render(
			<PaginationControls
				total={50}
				pageSize={10}
				offset={0}
				onOffsetChange={onOffsetChange}
				onPageSizeChange={() => {}}
			/>,
		);

		// With 50 items and 10 per page, there are 5 pages, all should be shown.
		await user.click(screen.getByRole("button", { name: "Page 3" }));
		expect(onOffsetChange).toHaveBeenCalledWith(20);
	});

	it("renders ellipsis when many pages", () => {
		render(
			<PaginationControls
				total={200}
				pageSize={10}
				offset={100}
				onOffsetChange={() => {}}
				onPageSizeChange={() => {}}
			/>,
		);

		expect(screen.getAllByText("...").length).toBeGreaterThan(0);
	});
});

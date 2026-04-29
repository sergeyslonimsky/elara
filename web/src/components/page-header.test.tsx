import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { PageHeader } from "./page-header";

describe("PageHeader", () => {
	it("renders title", () => {
		render(<PageHeader title="Test Title" />);
		expect(screen.getByText("Test Title")).toBeInTheDocument();
	});

	it("renders children in the right section", () => {
		render(
			<PageHeader title="Title">
				<button type="button">Action</button>
			</PageHeader>,
		);
		expect(screen.getByRole("button", { name: "Action" })).toBeInTheDocument();
	});

	it("renders refresh button when onRefresh is provided", async () => {
		const onRefresh = vi.fn();
		const user = userEvent.setup();
		render(<PageHeader title="Title" onRefresh={onRefresh} />);

		const refreshBtn = screen.getByRole("button", { name: "Refresh" });
		await user.click(refreshBtn);
		expect(onRefresh).toHaveBeenCalledTimes(1);
	});

	it("shows spinning icon when isRefreshing is true", () => {
		render(<PageHeader title="Title" onRefresh={() => {}} isRefreshing />);
		// The icon is inside the button
		const icon = screen
			.getByRole("button", { name: "Refresh" })
			.querySelector("svg");
		expect(icon).toHaveClass("animate-spin");
	});
});

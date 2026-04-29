import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { PageShell } from "./page-shell";

describe("PageShell", () => {
	it("renders title, header slot and children", () => {
		render(
			<PageShell
				title="My Page"
				headerSlot={<button type="button">Action</button>}
			>
				<div data-testid="content">Content</div>
			</PageShell>,
		);

		expect(screen.getByText("My Page")).toBeInTheDocument();
		expect(screen.getByRole("button", { name: "Action" })).toBeInTheDocument();
		expect(screen.getByTestId("content")).toBeInTheDocument();
	});

	it("applies custom contentClassName", () => {
		render(
			<PageShell title="Title" contentClassName="custom-gap">
				<div>Content</div>
			</PageShell>,
		);
		expect(screen.getByText("Content").parentElement).toHaveClass("custom-gap");
	});
});

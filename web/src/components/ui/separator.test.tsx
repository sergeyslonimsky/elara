import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { Separator } from "./separator";

describe("Separator", () => {
	it("renders with default horizontal orientation", () => {
		render(<Separator data-testid="separator" />);
		const separator = screen.getByTestId("separator");
		// Check for the orientation prop being handled correctly by Base UI
		expect(separator).toHaveAttribute("aria-orientation", "horizontal");
		// Check for the class name associated with horizontal orientation
		expect(separator).toHaveClass("data-horizontal:h-px");
	});

	it("renders with vertical orientation", () => {
		render(<Separator orientation="vertical" data-testid="separator" />);
		const separator = screen.getByTestId("separator");
		expect(separator).toHaveAttribute("aria-orientation", "vertical");
		expect(separator).toHaveClass("data-vertical:w-px");
	});

	it("applies custom className", () => {
		render(<Separator className="custom-class" data-testid="separator" />);
		const separator = screen.getByTestId("separator");
		expect(separator).toHaveClass("custom-class");
	});

	it("forwards other props", () => {
		render(<Separator id="test-id" data-testid="separator" />);
		const separator = screen.getByTestId("separator");
		expect(separator).toHaveAttribute("id", "test-id");
	});
});

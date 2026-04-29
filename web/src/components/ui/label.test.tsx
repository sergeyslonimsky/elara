import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { Label } from "./label";

describe("Label", () => {
	it("renders label element", () => {
		render(<Label>Label text</Label>);
		expect(screen.getByText("Label text")).toBeInTheDocument();
	});

	it("renders as label element", () => {
		render(<Label>Text</Label>);
		const label = screen.getByText("Text");
		expect(label.tagName).toBe("LABEL");
	});

	it("applies default classes", () => {
		render(<Label>Label</Label>);
		const label = screen.getByText("Label");
		expect(label).toHaveClass("text-sm");
		expect(label).toHaveClass("font-medium");
		expect(label).toHaveClass("leading-none");
	});

	it("applies select-none class", () => {
		render(<Label>Select me</Label>);
		const label = screen.getByText("Select me");
		expect(label).toHaveClass("select-none");
	});

	it("applies custom className", () => {
		render(<Label className="custom-label">Label</Label>);
		const label = screen.getByText("Label");
		expect(label).toHaveClass("custom-label");
	});

	it("forwards HTML attributes", () => {
		render(<Label data-testid="test-label">Label</Label>);
		expect(screen.getByTestId("test-label")).toBeInTheDocument();
	});

	it("forwards htmlFor attribute", () => {
		render(
			<>
				<Label htmlFor="test-input">Label</Label>
				<input id="test-input" />
			</>,
		);
		const label = screen.getByText("Label");
		expect(label).toHaveAttribute("for", "test-input");
	});

	it("applies disabled state classes when parent is disabled", () => {
		render(
			<div className="group" data-disabled="true">
				<Label>Disabled Label</Label>
			</div>,
		);
		const label = screen.getByText("Disabled Label");
		expect(label).toHaveClass("group-data-[disabled=true]:opacity-50");
	});
});

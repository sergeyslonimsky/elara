import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { Input } from "./input";

describe("Input", () => {
	it("renders input element", () => {
		render(<Input />);
		expect(screen.getByRole("textbox")).toBeInTheDocument();
	});

	it("applies default classes", () => {
		render(<Input />);
		const input = screen.getByRole("textbox");
		expect(input).toHaveClass("h-8");
		expect(input).toHaveClass("w-full");
		expect(input).toHaveClass("border-input");
		expect(input).toHaveClass("bg-transparent");
	});

	it("renders with type text", () => {
		render(<Input type="text" />);
		const input = screen.getByRole("textbox");
		expect(input).toHaveAttribute("type", "text");
	});

	it("renders with type email", () => {
		render(<Input type="email" />);
		const input = screen.getByRole("textbox");
		expect(input).toHaveAttribute("type", "email");
	});

	it("renders with type password", () => {
		render(<Input type="password" data-testid="input" />);
		const input = screen.getByTestId("input");
		expect(input).toHaveAttribute("type", "password");
	});

	it("renders with type number", () => {
		render(<Input type="number" />);
		const input = screen.getByRole("spinbutton");
		expect(input).toHaveAttribute("type", "number");
	});

	it("applies placeholder", () => {
		render(<Input placeholder="Enter text..." />);
		const input = screen.getByPlaceholderText("Enter text...");
		expect(input).toBeInTheDocument();
	});

	it("applies disabled state", () => {
		render(<Input disabled />);
		const input = screen.getByRole("textbox");
		expect(input).toBeDisabled();
		expect(input).toHaveClass("disabled:pointer-events-none");
	});

	it("applies custom className", () => {
		render(<Input className="custom-input" />);
		const input = screen.getByRole("textbox");
		expect(input).toHaveClass("custom-input");
	});

	it("forwards HTML attributes", () => {
		render(<Input data-testid="test-input" aria-label="Test input" />);
		expect(screen.getByTestId("test-input")).toBeInTheDocument();
		expect(screen.getByLabelText("Test input")).toBeInTheDocument();
	});

	it("applies focus-visible ring classes", () => {
		render(<Input />);
		const input = screen.getByRole("textbox");
		expect(input).toHaveClass("focus-visible:ring-3");
		expect(input).toHaveClass("focus-visible:ring-ring/50");
	});

	it("applies invalid state classes", () => {
		render(<Input aria-invalid="true" />);
		const input = screen.getByRole("textbox");
		expect(input).toHaveAttribute("aria-invalid", "true");
		expect(input).toHaveClass("aria-invalid:border-destructive");
	});
});

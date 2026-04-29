import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import { Textarea } from "./textarea";

describe("Textarea", () => {
	it("renders correctly", () => {
		render(<Textarea placeholder="Type here..." />);
		expect(screen.getByPlaceholderText("Type here...")).toBeInTheDocument();
	});

	it("allows typing", async () => {
		const user = userEvent.setup();
		render(<Textarea placeholder="Type here..." />);
		const textarea = screen.getByPlaceholderText("Type here...");

		await user.type(textarea, "Hello world");
		expect(textarea).toHaveValue("Hello world");
	});

	it("applies custom className", () => {
		render(<Textarea className="custom-class" />);
		expect(screen.getByRole("textbox")).toHaveClass("custom-class");
	});

	it("can be disabled", () => {
		render(<Textarea disabled />);
		expect(screen.getByRole("textbox")).toBeDisabled();
	});

	it("can be readOnly", () => {
		render(<Textarea readOnly />);
		expect(screen.getByRole("textbox")).toHaveAttribute("readonly");
	});

	it("forwards other attributes", () => {
		render(<Textarea data-testid="test-textarea" rows={5} />);
		const textarea = screen.getByTestId("test-textarea");
		expect(textarea).toHaveAttribute("rows", "5");
	});
});

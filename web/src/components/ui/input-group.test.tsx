import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import {
	InputGroup,
	InputGroupAddon,
	InputGroupButton,
	InputGroupInput,
	InputGroupText,
} from "./input-group";

describe("InputGroup", () => {
	it("renders children", () => {
		render(
			<InputGroup data-testid="group">
				<InputGroupAddon data-testid="addon">@</InputGroupAddon>
				<InputGroupInput placeholder="username" />
			</InputGroup>,
		);
		expect(screen.getByTestId("group")).toBeInTheDocument();
		expect(screen.getByText("@")).toBeInTheDocument();
		expect(screen.getByPlaceholderText("username")).toBeInTheDocument();
	});

	it("applies correct base classes", () => {
		render(<InputGroup data-testid="group" />);
		const group = screen.getByTestId("group");
		expect(group).toHaveClass("group/input-group");
		expect(group).toHaveClass("relative");
		expect(group).toHaveClass("flex");
	});

	it("renders InputGroupAddon with alignment", () => {
		const { rerender } = render(
			<InputGroupAddon data-testid="addon" align="inline-start">
				Start
			</InputGroupAddon>,
		);
		let addon = screen.getByTestId("addon");
		expect(addon).toHaveAttribute("data-align", "inline-start");
		expect(addon).toHaveClass("order-first");

		rerender(
			<InputGroupAddon data-testid="addon" align="inline-end">
				End
			</InputGroupAddon>,
		);
		addon = screen.getByTestId("addon");
		expect(addon).toHaveAttribute("data-align", "inline-end");
		expect(addon).toHaveClass("order-last");
	});

	it("renders InputGroupButton", () => {
		render(<InputGroupButton>Click</InputGroupButton>);
		const button = screen.getByRole("button", { name: "Click" });
		expect(button).toBeInTheDocument();
		// The custom InputGroupButton component sets data-size
		expect(button).toHaveAttribute("data-size", "xs");
	});

	it("renders InputGroupText", () => {
		render(<InputGroupText>Text</InputGroupText>);
		expect(screen.getByText("Text")).toBeInTheDocument();
	});

	it("forwards custom className", () => {
		render(<InputGroup className="custom-group" data-testid="group" />);
		expect(screen.getByTestId("group")).toHaveClass("custom-group");
	});
});

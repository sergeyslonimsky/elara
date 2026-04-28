import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import { Checkbox } from "./checkbox";

describe("Checkbox", () => {
	it("renders correctly", () => {
		render(<Checkbox aria-label="test-checkbox" />);
		expect(screen.getByRole("checkbox")).toBeInTheDocument();
	});

	it("can be checked and unchecked", async () => {
		const user = userEvent.setup();
		render(<Checkbox aria-label="test-checkbox" />);
		const checkbox = screen.getByRole("checkbox");

		expect(checkbox).not.toBeChecked();

		await user.click(checkbox);
		expect(checkbox).toBeChecked();

		await user.click(checkbox);
		expect(checkbox).not.toBeChecked();
	});

	it("applies custom className", () => {
		render(<Checkbox className="custom-class" aria-label="test-checkbox" />);
		expect(screen.getByRole("checkbox")).toHaveClass("custom-class");
	});

	it("can be disabled", () => {
		render(<Checkbox disabled aria-label="test-checkbox" />);
		const checkbox = screen.getByRole("checkbox");
		expect(checkbox).toHaveAttribute("aria-disabled", "true");
		expect(checkbox).toHaveAttribute("data-disabled");
	});

	it("can be required", () => {
		render(<Checkbox required aria-label="test-checkbox" />);
		expect(screen.getByRole("checkbox")).toBeRequired();
	});
});

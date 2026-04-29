import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { Logo } from "./logo";

describe("Logo", () => {
	it("renders correctly with accessibility label", () => {
		render(<Logo />);
		expect(screen.getByRole("img", { name: "Elara" })).toBeInTheDocument();
	});

	it("applies custom className", () => {
		render(<Logo className="custom-logo" />);
		expect(screen.getByRole("img")).toHaveClass("custom-logo");
	});
});

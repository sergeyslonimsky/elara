import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { Skeleton } from "./skeleton";

describe("Skeleton", () => {
	it("renders with default classes", () => {
		render(<Skeleton data-testid="skeleton" />);
		const skeleton = screen.getByTestId("skeleton");
		expect(skeleton).toHaveClass("animate-pulse");
		expect(skeleton).toHaveClass("rounded-md");
		expect(skeleton).toHaveClass("bg-muted");
	});

	it("applies custom className", () => {
		render(<Skeleton className="custom-class" data-testid="skeleton" />);
		const skeleton = screen.getByTestId("skeleton");
		expect(skeleton).toHaveClass("custom-class");
	});

	it("forwards other props", () => {
		render(<Skeleton id="test-id" data-testid="skeleton" />);
		const skeleton = screen.getByTestId("skeleton");
		expect(skeleton).toHaveAttribute("id", "test-id");
	});
});

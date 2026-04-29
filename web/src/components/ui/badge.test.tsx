import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { Badge } from "./badge";

describe("Badge", () => {
	it("renders children", () => {
		render(<Badge>New</Badge>);
		expect(screen.getByText("New")).toBeInTheDocument();
	});

	it("applies default variant", () => {
		render(<Badge data-testid="badge">Default</Badge>);
		const badge = screen.getByTestId("badge");
		expect(badge).toHaveClass("group/badge");
	});

	it("applies secondary variant", () => {
		render(<Badge variant="secondary" data-testid="badge">Secondary</Badge>);
		const badge = screen.getByTestId("badge");
		expect(badge).toHaveClass("bg-secondary");
	});

	it("applies destructive variant", () => {
		render(<Badge variant="destructive" data-testid="badge">Destructive</Badge>);
		const badge = screen.getByTestId("badge");
		expect(badge).toHaveClass("bg-destructive/10");
	});

	it("applies outline variant", () => {
		render(<Badge variant="outline" data-testid="badge">Outline</Badge>);
		const badge = screen.getByTestId("badge");
		expect(badge).toHaveClass("border-border");
	});

	it("applies ghost variant", () => {
		render(<Badge variant="ghost" data-testid="badge">Ghost</Badge>);
		const badge = screen.getByTestId("badge");
		expect(badge).toHaveClass("hover:bg-muted");
	});

	it("applies success variant", () => {
		render(<Badge variant="success" data-testid="badge">Success</Badge>);
		const badge = screen.getByTestId("badge");
		expect(badge).toHaveClass("bg-emerald-100");
		expect(badge).toHaveClass("text-emerald-700");
	});

	it("applies info variant", () => {
		render(<Badge variant="info" data-testid="badge">Info</Badge>);
		const badge = screen.getByTestId("badge");
		expect(badge).toHaveClass("bg-blue-100");
		expect(badge).toHaveClass("text-blue-700");
	});

	it("applies warning variant", () => {
		render(<Badge variant="warning" data-testid="badge">Warning</Badge>);
		const badge = screen.getByTestId("badge");
		expect(badge).toHaveClass("bg-amber-100");
		expect(badge).toHaveClass("text-amber-700");
	});

	it("applies custom className", () => {
		render(<Badge className="custom-badge" data-testid="badge">Custom</Badge>);
		const badge = screen.getByTestId("badge");
		expect(badge).toHaveClass("custom-badge");
	});

	it("forwards HTML attributes", () => {
		render(<Badge data-testid="test-badge">Test</Badge>);
		expect(screen.getByTestId("test-badge")).toBeInTheDocument();
	});

	it("renders as anchor when render prop is provided", () => {
		render(<Badge render={<a href="/link" />}>Link Badge</Badge>);
		const link = screen.getByRole("link");
		expect(link).toHaveAttribute("href", "/link");
		expect(link).toHaveClass("group/badge");
	});

	it("has correct size classes", () => {
		render(<Badge data-testid="badge">Size Test</Badge>);
		const badge = screen.getByTestId("badge");
		expect(badge).toHaveClass("h-5");
		expect(badge).toHaveClass("text-xs");
		expect(badge).toHaveClass("px-2");
	});
});

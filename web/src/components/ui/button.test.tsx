import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { Button } from "./button";

describe("Button", () => {
	it("renders children", () => {
		render(<Button>Click me</Button>);
		expect(screen.getByRole("button", { name: "Click me" })).toBeInTheDocument();
	});

	it("applies default variant", () => {
		render(<Button>Default</Button>);
		const button = screen.getByRole("button");
		expect(button).toHaveClass("bg-primary");
	});

	it("applies outline variant", () => {
		render(<Button variant="outline">Outline</Button>);
		const button = screen.getByRole("button");
		expect(button).toHaveClass("border-border");
	});

	it("applies destructive variant", () => {
		render(<Button variant="destructive">Destructive</Button>);
		const button = screen.getByRole("button");
		expect(button).toHaveClass("bg-destructive/10");
	});

	it("applies ghost variant", () => {
		render(<Button variant="ghost">Ghost</Button>);
		const button = screen.getByRole("button");
		expect(button).toHaveClass("hover:bg-muted");
	});

	it("applies link variant", () => {
		render(<Button variant="link">Link</Button>);
		const button = screen.getByRole("button");
		expect(button).toHaveClass("underline-offset-4");
	});

	it("applies default size", () => {
		render(<Button>Default Size</Button>);
		const button = screen.getByRole("button");
		expect(button).toHaveClass("h-8");
	});

	it("applies sm size", () => {
		render(<Button size="sm">Small</Button>);
		const button = screen.getByRole("button");
		expect(button).toHaveClass("h-7");
	});

	it("applies lg size", () => {
		render(<Button size="lg">Large</Button>);
		const button = screen.getByRole("button");
		expect(button).toHaveClass("h-9");
	});

	it("applies icon size", () => {
		render(<Button size="icon" aria-label="Icon button" />);
		const button = screen.getByRole("button");
		expect(button).toHaveClass("size-8");
	});

	it("applies disabled state", () => {
		render(<Button disabled>Disabled</Button>);
		const button = screen.getByRole("button");
		expect(button).toBeDisabled();
		expect(button).toHaveClass("disabled:pointer-events-none");
	});

	it("forwards className", () => {
		render(<Button className="custom-class">Custom</Button>);
		const button = screen.getByRole("button");
		expect(button).toHaveClass("custom-class");
	});

	it("forwards HTML attributes", () => {
		render(<Button data-testid="test-button">Test</Button>);
		expect(screen.getByTestId("test-button")).toBeInTheDocument();
	});

	it("renders as link when render prop is provided", () => {
		render(<Button render={<a href="/test" />}>Link Button</Button>);
		const link = screen.getByRole("link");
		expect(link).toHaveAttribute("href", "/test");
		expect(link).toHaveClass("group/button");
	});
});

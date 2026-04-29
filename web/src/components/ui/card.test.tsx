import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import {
	Card,
	CardHeader,
	CardTitle,
	CardDescription,
	CardContent,
	CardFooter,
	CardAction,
} from "./card";

describe("Card", () => {
	it("renders card with default size", () => {
		render(
			<Card data-testid="card">
				<span>Content</span>
			</Card>,
		);
		const card = screen.getByTestId("card");
		expect(card).toHaveClass("py-4");
		expect(card).toHaveAttribute("data-size", "default");
	});

	it("renders card with sm size", () => {
		render(
			<Card size="sm" data-testid="card">
				<span>Small Card</span>
			</Card>,
		);
		const card = screen.getByTestId("card");
		expect(card).toHaveAttribute("data-size", "sm");
	});

	it("applies custom className", () => {
		render(
			<Card className="custom-card" data-testid="card">
				<span>Content</span>
			</Card>,
		);
		const card = screen.getByTestId("card");
		expect(card).toHaveClass("custom-card");
	});

	it("forwards HTML attributes", () => {
		render(
			<Card data-testid="test-card">
				<span>Content</span>
			</Card>,
		);
		expect(screen.getByTestId("test-card")).toBeInTheDocument();
	});
});

describe("CardHeader", () => {
	it("renders header content", () => {
		render(<CardHeader>Header</CardHeader>);
		expect(screen.getByText("Header")).toBeInTheDocument();
	});

	it("applies correct classes", () => {
		render(<CardHeader data-testid="header">Header</CardHeader>);
		const header = screen.getByTestId("header");
		expect(header).toHaveClass("group/card-header");
		expect(header).toHaveAttribute("data-slot", "card-header");
	});

	it("applies custom className", () => {
		render(<CardHeader className="custom-header" data-testid="header">Header</CardHeader>);
		const header = screen.getByTestId("header");
		expect(header).toHaveClass("custom-header");
	});
});

describe("CardTitle", () => {
	it("renders title content", () => {
		render(<CardTitle>Title</CardTitle>);
		expect(screen.getByText("Title")).toBeInTheDocument();
	});

	it("applies font classes", () => {
		render(<CardTitle data-testid="title">Title</CardTitle>);
		const title = screen.getByTestId("title");
		expect(title).toHaveClass("font-heading");
		expect(title).toHaveClass("font-medium");
	});

	it("applies custom className", () => {
		render(<CardTitle className="custom-title" data-testid="title">Title</CardTitle>);
		const title = screen.getByTestId("title");
		expect(title).toHaveClass("custom-title");
	});
});

describe("CardDescription", () => {
	it("renders description content", () => {
		render(<CardDescription>Description text</CardDescription>);
		expect(screen.getByText("Description text")).toBeInTheDocument();
	});

	it("applies muted text class", () => {
		render(<CardDescription data-testid="desc">Description</CardDescription>);
		const desc = screen.getByTestId("desc");
		expect(desc).toHaveClass("text-muted-foreground");
	});

	it("applies custom className", () => {
		render(<CardDescription className="custom-desc" data-testid="desc">Description</CardDescription>);
		const desc = screen.getByTestId("desc");
		expect(desc).toHaveClass("custom-desc");
	});
});

describe("CardContent", () => {
	it("renders content", () => {
		render(<CardContent>Card content here</CardContent>);
		expect(screen.getByText("Card content here")).toBeInTheDocument();
	});

	it("applies padding classes", () => {
		render(<CardContent data-testid="content">Content</CardContent>);
		const content = screen.getByTestId("content");
		expect(content).toHaveClass("px-4");
	});

	it("applies custom className", () => {
		render(<CardContent className="custom-content" data-testid="content">Content</CardContent>);
		const content = screen.getByTestId("content");
		expect(content).toHaveClass("custom-content");
	});
});

describe("CardFooter", () => {
	it("renders footer content", () => {
		render(<CardFooter>Footer</CardFooter>);
		expect(screen.getByText("Footer")).toBeInTheDocument();
	});

	it("applies border and background classes", () => {
		render(<CardFooter data-testid="footer">Footer</CardFooter>);
		const footer = screen.getByTestId("footer");
		expect(footer).toHaveClass("border-t");
		expect(footer).toHaveClass("bg-muted/50");
	});

	it("applies custom className", () => {
		render(<CardFooter className="custom-footer" data-testid="footer">Footer</CardFooter>);
		const footer = screen.getByTestId("footer");
		expect(footer).toHaveClass("custom-footer");
	});
});

describe("CardAction", () => {
	it("renders action content", () => {
		render(<CardAction>Action</CardAction>);
		expect(screen.getByText("Action")).toBeInTheDocument();
	});

	it("applies positioning classes", () => {
		render(<CardAction data-testid="action">Action</CardAction>);
		const action = screen.getByTestId("action");
		expect(action).toHaveClass("col-start-2");
		expect(action).toHaveClass("row-span-2");
	});

	it("applies custom className", () => {
		render(<CardAction className="custom-action" data-testid="action">Action</CardAction>);
		const action = screen.getByTestId("action");
		expect(action).toHaveClass("custom-action");
	});
});

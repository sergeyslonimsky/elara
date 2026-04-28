import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "./tabs";

describe("Tabs", () => {
	it("renders tabs and content", () => {
		render(
			<Tabs defaultValue="tab1">
				<TabsList>
					<TabsTrigger value="tab1">Tab 1</TabsTrigger>
					<TabsTrigger value="tab2">Tab 2</TabsTrigger>
				</TabsList>
				<TabsContent value="tab1">Content 1</TabsContent>
				<TabsContent value="tab2">Content 2</TabsContent>
			</Tabs>,
		);

		expect(screen.getByRole("tab", { name: "Tab 1" })).toBeInTheDocument();
		expect(screen.getByText("Content 1")).toBeInTheDocument();
	});

	it("applies horizontal orientation by default to Tabs root", () => {
		render(
			<Tabs data-testid="tabs">
				<TabsList />
			</Tabs>,
		);
		const tabs = screen.getByTestId("tabs");
		expect(tabs).toHaveAttribute("data-orientation", "horizontal");
		expect(tabs).toHaveClass("data-horizontal:flex-col");
	});

	it("applies vertical orientation to Tabs root", () => {
		render(
			<Tabs orientation="vertical" data-testid="tabs">
				<TabsList />
			</Tabs>,
		);
		const tabs = screen.getByTestId("tabs");
		expect(tabs).toHaveAttribute("data-orientation", "vertical");
	});

	it("renders TabsList with default variant", () => {
		render(
			<Tabs>
				<TabsList data-testid="tabs-list" />
			</Tabs>
		);
		const tabsList = screen.getByTestId("tabs-list");
		expect(tabsList).toHaveAttribute("data-variant", "default");
		expect(tabsList).toHaveClass("bg-muted");
	});

	it("renders TabsList with line variant", () => {
		render(
			<Tabs>
				<TabsList variant="line" data-testid="tabs-list" />
			</Tabs>
		);
		const tabsList = screen.getByTestId("tabs-list");
		expect(tabsList).toHaveAttribute("data-variant", "line");
		expect(tabsList).toHaveClass("bg-transparent");
	});

	it("forwards className to all components", () => {
		render(
			<Tabs className="custom-tabs" data-testid="tabs">
				<TabsList className="custom-list" data-testid="list">
					<TabsTrigger value="t1" className="custom-trigger" />
				</TabsList>
				<TabsContent value="t1" className="custom-content" data-testid="content">
					Content
				</TabsContent>
			</Tabs>,
		);

		expect(screen.getByTestId("tabs")).toHaveClass("custom-tabs");
		expect(screen.getByTestId("list")).toHaveClass("custom-list");
		expect(screen.getByRole("tab")).toHaveClass("custom-trigger");
		expect(screen.getByTestId("content")).toHaveClass("custom-content");
	});
});

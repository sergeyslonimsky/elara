import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import {
	Breadcrumb,
	BreadcrumbEllipsis,
	BreadcrumbItem,
	BreadcrumbLink,
	BreadcrumbList,
	BreadcrumbPage,
	BreadcrumbSeparator,
} from "./breadcrumb";

describe("Breadcrumb", () => {
	it("renders correctly with all parts", () => {
		render(
			<Breadcrumb>
				<BreadcrumbList>
					<BreadcrumbItem>
						<BreadcrumbLink href="/">Home</BreadcrumbLink>
					</BreadcrumbItem>
					<BreadcrumbSeparator />
					<BreadcrumbItem>
						<BreadcrumbEllipsis />
					</BreadcrumbItem>
					<BreadcrumbSeparator />
					<BreadcrumbItem>
						<BreadcrumbPage>Current Page</BreadcrumbPage>
					</BreadcrumbItem>
				</BreadcrumbList>
			</Breadcrumb>,
		);

		expect(screen.getByRole("navigation", { name: "breadcrumb" })).toBeInTheDocument();
		expect(screen.getByRole("link", { name: "Home" })).toBeInTheDocument();
		expect(screen.getByText("Current Page")).toBeInTheDocument();
		expect(screen.getByText("More")).toBeInTheDocument(); // sr-only text in ellipsis
	});

	it("renders custom separator", () => {
		render(
			<Breadcrumb>
				<BreadcrumbList>
					<BreadcrumbItem>
						<BreadcrumbLink href="/">Home</BreadcrumbLink>
					</BreadcrumbItem>
					<BreadcrumbSeparator>/</BreadcrumbSeparator>
					<BreadcrumbItem>
						<BreadcrumbPage>Current</BreadcrumbPage>
					</BreadcrumbItem>
				</BreadcrumbList>
			</Breadcrumb>,
		);

		expect(screen.getByText("/")).toBeInTheDocument();
	});

	it("renders as different element when 'render' prop is used in BreadcrumbLink", () => {
		render(
			<BreadcrumbLink render={<button type="button" />}>
				Button Link
			</BreadcrumbLink>
		);
		expect(screen.getByRole("button", { name: "Button Link" })).toBeInTheDocument();
	});
});

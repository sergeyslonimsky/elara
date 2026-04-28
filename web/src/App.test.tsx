import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import App from "./App";
import { MemoryRouter } from "react-router";
import { TestProviders } from "@/test/test-utils";

describe("App", () => {
	it("renders the dashboard page at root route", () => {
		render(
			<MemoryRouter initialEntries={["/"]}>
				<TestProviders>
					<App />
				</TestProviders>
			</MemoryRouter>,
		);

		expect(
			screen.getByRole("heading", { name: "Dashboard" }),
		).toBeInTheDocument();
	});

	it("renders the 404 page for unknown routes", () => {
		render(
			<MemoryRouter initialEntries={["/unknown-route"]}>
				<TestProviders>
					<App />
				</TestProviders>
			</MemoryRouter>,
		);

		expect(screen.getByText("Page not found")).toBeInTheDocument();
		expect(
			screen.getByText("The page you're looking for doesn't exist."),
		).toBeInTheDocument();
	});

	it("renders navigation sidebar", () => {
		render(
			<MemoryRouter initialEntries={["/"]}>
				<TestProviders>
					<App />
				</TestProviders>
			</MemoryRouter>,
		);

		expect(
			screen.getAllByRole("button", { name: "Toggle Sidebar" }),
		).toHaveLength(2);
	});

	it("renders app header", () => {
		render(
			<MemoryRouter initialEntries={["/"]}>
				<TestProviders>
					<App />
				</TestProviders>
			</MemoryRouter>,
		);

		expect(screen.getByRole("banner")).toBeInTheDocument();
	});
});

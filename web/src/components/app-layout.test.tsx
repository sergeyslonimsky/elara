import { render, screen } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router";
import { describe, expect, it, vi } from "vitest";
import { AppLayout } from "./app-layout";

vi.mock("@/components/app-header", () => ({
	AppHeader: () => <div data-testid="app-header" />,
}));

vi.mock("@/components/app-sidebar", () => ({
	AppSidebar: () => <div data-testid="app-sidebar" />,
}));

describe("AppLayout", () => {
	it("renders sidebar, header and outlet content", () => {
		render(
			<MemoryRouter initialEntries={["/"]}>
				<Routes>
					<Route element={<AppLayout />}>
						<Route
							path="/"
							element={<div data-testid="child">Child Content</div>}
						/>
					</Route>
				</Routes>
			</MemoryRouter>,
		);

		expect(screen.getByTestId("app-sidebar")).toBeInTheDocument();
		expect(screen.getByTestId("app-header")).toBeInTheDocument();
		expect(screen.getByTestId("child")).toBeInTheDocument();
	});
});

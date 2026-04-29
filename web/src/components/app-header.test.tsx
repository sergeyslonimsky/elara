import { render, screen } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router";
import { describe, expect, it } from "vitest";
import { SidebarProvider } from "@/components/ui/sidebar";
import { TestProviders } from "@/test/test-utils";
import { AppHeader } from "./app-header";

describe("AppHeader", () => {
	it("renders sidebar trigger and theme toggle", () => {
		render(
			<TestProviders>
				<SidebarProvider>
					<MemoryRouter>
						<AppHeader />
					</MemoryRouter>
				</SidebarProvider>
			</TestProviders>,
		);

		expect(
			screen.getByRole("button", { name: "Toggle Sidebar" }),
		).toBeInTheDocument();
		expect(
			screen.getByRole("button", { name: "Toggle theme" }),
		).toBeInTheDocument();
	});

	it("shows namespace badge when namespace param is present", () => {
		render(
			<TestProviders>
				<SidebarProvider>
					<MemoryRouter initialEntries={["/ns/test-namespace"]}>
						<Routes>
							<Route path="/ns/:namespace" element={<AppHeader />} />
						</Routes>
					</MemoryRouter>
				</SidebarProvider>
			</TestProviders>,
		);

		expect(screen.getByText("test-namespace")).toBeInTheDocument();
	});

	it("does not show namespace badge when namespace param is absent", () => {
		render(
			<TestProviders>
				<SidebarProvider>
					<MemoryRouter initialEntries={["/"]}>
						<Routes>
							<Route path="/" element={<AppHeader />} />
						</Routes>
					</MemoryRouter>
				</SidebarProvider>
			</TestProviders>,
		);

		// The badge with namespace should not be there.
		// Since it's a Badge component with font-mono text-xs, we check if test-namespace is NOT there.
		expect(screen.queryByText("test-namespace")).not.toBeInTheDocument();
	});
});

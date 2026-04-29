import { render, screen } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router";
import { describe, expect, it } from "vitest";
import { SidebarProvider } from "@/components/ui/sidebar";
import { AppSidebar } from "./app-sidebar";

describe("AppSidebar", () => {
	it("renders navigation items", () => {
		render(
			<SidebarProvider>
				<MemoryRouter>
					<AppSidebar />
				</MemoryRouter>
			</SidebarProvider>,
		);

		expect(screen.getByText("Dashboard")).toBeInTheDocument();
		expect(screen.getByText("Configs")).toBeInTheDocument();
		expect(screen.getByText("Namespaces")).toBeInTheDocument();
		expect(screen.getByText("Clients")).toBeInTheDocument();
		expect(screen.getByText("Webhooks")).toBeInTheDocument();
	});

	it("highlights active item based on pathname", () => {
		render(
			<SidebarProvider>
				<MemoryRouter initialEntries={["/namespaces"]}>
					<AppSidebar />
				</MemoryRouter>
			</SidebarProvider>,
		);

		const namespacesLink = screen.getByRole("link", { name: "Namespaces" });
		// SidebarMenuButton sets data-active={isActive} via useRender.
		// Boolean attributes in useRender results in [data-active] being present/absent.
		expect(namespacesLink).toHaveAttribute("data-active");

		const dashboardLink = screen.getByRole("link", { name: "Dashboard" });
		expect(dashboardLink).not.toHaveAttribute("data-active");
	});

	it("adjusts Configs link when namespace is present", () => {
		render(
			<SidebarProvider>
				<MemoryRouter initialEntries={["/browse/my-ns"]}>
					<Routes>
						<Route path="/browse/:namespace" element={<AppSidebar />} />
					</Routes>
				</MemoryRouter>
			</SidebarProvider>,
		);

		const configsLink = screen.getByRole("link", { name: "Configs" });
		expect(configsLink).toHaveAttribute("href", "/browse/my-ns");
	});
});

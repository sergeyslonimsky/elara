import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { Sidebar, SidebarProvider, SidebarContent, SidebarTrigger } from "./sidebar";

// Mock useIsMobile hook
vi.mock("@/hooks/use-mobile", () => ({
	useIsMobile: vi.fn(() => false),
}));

describe("Sidebar", () => {
	it("renders correctly", () => {
		render(
			<SidebarProvider>
				<Sidebar>
					<SidebarContent>
						<div data-testid="sidebar-item">Item</div>
					</SidebarContent>
				</Sidebar>
				<SidebarTrigger />
			</SidebarProvider>
		);

		expect(screen.getByTestId("sidebar-item")).toBeInTheDocument();
		expect(screen.getByRole("button", { name: "Toggle Sidebar" })).toBeInTheDocument();
	});

	it("toggles state", async () => {
		const { userEvent } = await import("@testing-library/user-event");
		const user = userEvent.setup();
		
		render(
			<SidebarProvider defaultOpen={true}>
				<Sidebar collapsible="icon" data-testid="sidebar">
					<SidebarContent>
						<div data-testid="sidebar-item">Item</div>
					</SidebarContent>
				</Sidebar>
				<SidebarTrigger />
			</SidebarProvider>
		);

		const trigger = screen.getByRole("button", { name: "Toggle Sidebar" });
		const sidebar = screen.getByTestId("sidebar");

		expect(sidebar).toHaveAttribute("data-state", "expanded");

		await user.click(trigger);
		expect(sidebar).toHaveAttribute("data-state", "collapsed");
	});
});

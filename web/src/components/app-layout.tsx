import { Outlet } from "react-router";
import { AppHeader } from "@/components/app-header";
import { AppSidebar } from "@/components/app-sidebar";
import { SidebarInset, SidebarProvider } from "@/components/ui/sidebar";

export function AppLayout() {
	return (
		<SidebarProvider>
			<AppSidebar />
			<SidebarInset>
				<AppHeader />
				<Outlet />
			</SidebarInset>
		</SidebarProvider>
	);
}

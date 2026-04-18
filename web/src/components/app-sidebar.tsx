import { Database, FolderTree, LayoutDashboard, Network } from "lucide-react";
import { Link, useLocation, useParams } from "react-router";
import { Logo } from "@/components/logo";
import {
	Sidebar,
	SidebarContent,
	SidebarGroup,
	SidebarGroupContent,
	SidebarGroupLabel,
	SidebarHeader,
	SidebarMenu,
	SidebarMenuButton,
	SidebarMenuItem,
	SidebarRail,
} from "@/components/ui/sidebar";

export function AppSidebar() {
	const location = useLocation();
	const { namespace } = useParams();

	const configsHref = namespace ? `/browse/${namespace}` : "/browse";

	const isDashboardActive = location.pathname === "/dashboard";
	const isConfigsActive =
		location.pathname.startsWith("/browse") ||
		location.pathname.startsWith("/config");
	const isNamespacesActive = location.pathname.startsWith("/namespaces");
	const isClientsActive = location.pathname.startsWith("/clients");

	return (
		<Sidebar>
			<SidebarHeader>
				<div className="flex items-center gap-2 px-2 py-1">
					<Logo className="h-7 w-7 text-primary" />
					<span className="font-semibold text-sm">Elara</span>
				</div>
			</SidebarHeader>
			<SidebarContent>
				<SidebarGroup>
					<SidebarGroupLabel>Navigation</SidebarGroupLabel>
					<SidebarGroupContent>
						<SidebarMenu>
							<SidebarMenuItem>
								<SidebarMenuButton
									isActive={isDashboardActive}
									render={<Link to="/dashboard" />}
								>
									<LayoutDashboard className="h-4 w-4" />
									<span>Dashboard</span>
								</SidebarMenuButton>
							</SidebarMenuItem>
							<SidebarMenuItem>
								<SidebarMenuButton
									isActive={isConfigsActive}
									render={<Link to={configsHref} />}
								>
									<FolderTree className="h-4 w-4" />
									<span>Configs</span>
								</SidebarMenuButton>
							</SidebarMenuItem>
							<SidebarMenuItem>
								<SidebarMenuButton
									isActive={isNamespacesActive}
									render={<Link to="/namespaces" />}
								>
									<Database className="h-4 w-4" />
									<span>Namespaces</span>
								</SidebarMenuButton>
							</SidebarMenuItem>
							<SidebarMenuItem>
								<SidebarMenuButton
									isActive={isClientsActive}
									render={<Link to="/clients" />}
								>
									<Network className="h-4 w-4" />
									<span>Clients</span>
								</SidebarMenuButton>
							</SidebarMenuItem>
						</SidebarMenu>
					</SidebarGroupContent>
				</SidebarGroup>
			</SidebarContent>
			<SidebarRail />
		</Sidebar>
	);
}

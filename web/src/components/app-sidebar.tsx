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
	const { pathname } = useLocation();
	const { namespace } = useParams();

	const navItems = [
		{
			title: "Dashboard",
			href: "/",
			icon: LayoutDashboard,
			isActive: pathname === "/",
		},
		{
			title: "Configs",
			href: namespace ? `/browse/${namespace}` : "/browse",
			icon: FolderTree,
			isActive:
				pathname.startsWith("/browse") || pathname.startsWith("/config"),
		},
		{
			title: "Namespaces",
			href: "/namespaces",
			icon: Database,
			isActive: pathname.startsWith("/namespaces"),
		},
		{
			title: "Clients",
			href: "/clients",
			icon: Network,
			isActive: pathname.startsWith("/clients"),
		},
	];

	return (
		<Sidebar>
			<SidebarHeader>
				<div className="flex items-center gap-2 px-2 py-1">
					<Logo className="h-7 w-7 text-foreground" />
					<span className="font-semibold text-sm">Elara</span>
				</div>
			</SidebarHeader>
			<SidebarContent>
				<SidebarGroup>
					<SidebarGroupLabel>Navigation</SidebarGroupLabel>
					<SidebarGroupContent>
						<SidebarMenu>
							{navItems.map((item) => (
								<SidebarMenuItem key={item.title}>
									<SidebarMenuButton
										isActive={item.isActive}
										render={<Link to={item.href} />}
									>
										<item.icon />
										<span>{item.title}</span>
									</SidebarMenuButton>
								</SidebarMenuItem>
							))}
						</SidebarMenu>
					</SidebarGroupContent>
				</SidebarGroup>
			</SidebarContent>
			<SidebarRail />
		</Sidebar>
	);
}

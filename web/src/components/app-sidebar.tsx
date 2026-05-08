import {
	ChevronDown,
	Database,
	FolderTree,
	Key,
	LayoutDashboard,
	Network,
	Shield,
	Users,
	UsersRound,
	Webhook,
} from "lucide-react";
import { useState } from "react";
import { Link, useLocation, useParams } from "react-router";
import { useAuth } from "@/components/auth-provider";
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
import { AuthType } from "@/gen/elara/auth/v1/auth_service_pb";

export function AppSidebar() {
	const { pathname } = useLocation();
	const { namespace } = useParams();
	const { authType, me } = useAuth();
	const [adminOpen, setAdminOpen] = useState(true);

	const navItems = [
		{
			title: "Dashboard",
			href: "/",
			icon: LayoutDashboard,
			isActive: pathname === "/",
			show: true,
		},
		{
			title: "Configs",
			href: namespace ? `/browse/${namespace}` : "/browse",
			icon: FolderTree,
			isActive:
				pathname.startsWith("/browse") || pathname.startsWith("/config"),
			show: true,
		},
		{
			title: "Namespaces",
			href: "/namespaces",
			icon: Database,
			isActive: pathname.startsWith("/namespaces"),
			show: true,
		},
		{
			title: "Clients",
			href: "/clients",
			icon: Network,
			isActive: pathname.startsWith("/clients"),
			show: true,
		},
		{
			title: "Webhooks",
			href: "/webhooks",
			icon: Webhook,
			isActive: pathname.startsWith("/webhooks"),
			show: me?.canViewWebhooks ?? false,
		},
		{
			title: "Tokens",
			href: "/tokens",
			icon: Key,
			isActive: pathname.startsWith("/tokens"),
			show: !!me,
		},
	].filter((item) => item.show);

	const administrationItems = [
		{
			title: "Users",
			href: "/users",
			icon: Users,
			isActive: pathname.startsWith("/users"),
		},
		{
			title: "Groups",
			href: "/groups",
			icon: UsersRound,
			isActive: pathname.startsWith("/groups"),
		},
		{
			title: "Access",
			href: "/access",
			icon: Shield,
			isActive: pathname.startsWith("/access"),
		},
	];

	const showAdministration = authType !== AuthType.NONE && me?.isAdmin === true;

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

				{showAdministration && (
					<SidebarGroup>
						<SidebarGroupLabel
							className="flex cursor-pointer items-center justify-between hover:text-foreground"
							onClick={() => setAdminOpen(!adminOpen)}
						>
							Administration
							<ChevronDown
								className={`h-3.5 w-3.5 transition-transform duration-200 ${
									adminOpen ? "" : "-rotate-90"
								}`}
							/>
						</SidebarGroupLabel>
						{adminOpen && (
							<SidebarGroupContent>
								<SidebarMenu>
									{administrationItems.map((item) => (
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
						)}
					</SidebarGroup>
				)}
			</SidebarContent>
			<SidebarRail />
		</Sidebar>
	);
}

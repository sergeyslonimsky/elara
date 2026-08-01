import { create } from "@bufbuild/protobuf";
import {
	ChevronDown,
	Database,
	FolderTree,
	Key,
	LayoutDashboard,
	Network,
	Users,
	UsersRound,
	Webhook,
} from "lucide-react";
import { useState } from "react";
import { Link, useLocation, useParams } from "react-router";
import { useAbility } from "@/auth/ability-context";
import { uiVisibility } from "@/auth/uiVisibility";
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
import { AuthType } from "@/gen/elara/auth/v1/auth_pb";
import { GetCapabilitiesResponseSchema } from "@/gen/elara/capabilities/v1/capabilities_service_pb";

export function AppSidebar() {
	const { pathname } = useLocation();
	const { namespace } = useParams();
	const { state } = useAuth();
	const ability = useAbility();
	const [adminOpen, setAdminOpen] = useState(true);

	const authType =
		state.status === "authenticated" ? state.authType : undefined;
	const capabilities =
		state.status === "authenticated" ? state.capabilities : undefined;

	const visibility = uiVisibility(
		ability,
		capabilities ?? create(GetCapabilitiesResponseSchema, {}),
	);

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
			show: visibility.canSeeConfigsSection,
		},
		{
			title: "Namespaces",
			href: "/namespaces",
			icon: Database,
			isActive: pathname.startsWith("/namespaces"),
			show: visibility.canSeeNamespacesSection,
		},
		{
			title: "Clients",
			href: "/clients",
			icon: Network,
			isActive: pathname.startsWith("/clients"),
			show: visibility.canSeeClientsSection,
		},
		{
			title: "Webhooks",
			href: "/webhooks",
			icon: Webhook,
			isActive: pathname.startsWith("/webhooks"),
			show: visibility.canSeeWebhooksSection,
		},
		{
			title: "Tokens",
			href: "/tokens",
			icon: Key,
			isActive: pathname.startsWith("/tokens"),
			show: visibility.canSeeTokensSection,
		},
	].filter((item) => item.show);

	const administrationItems = [
		{
			title: "Users",
			href: "/users",
			icon: Users,
			isActive: pathname.startsWith("/users"),
			show: visibility.canSeeUsersSection,
		},
		{
			title: "Groups",
			href: "/groups",
			icon: UsersRound,
			isActive: pathname.startsWith("/groups"),
			show: visibility.canSeeGroupsSection,
		},
	].filter((item) => item.show);

	const showAdministration =
		authType !== AuthType.NONE && administrationItems.length > 0;

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

import { create } from "@bufbuild/protobuf";
import { render, screen } from "@testing-library/react";
import { Route, Routes } from "react-router";
import { describe, expect, it } from "vitest";
import { buildAbility } from "@/auth/ability";
import type { AuthContextType } from "@/components/auth-provider";
import { SidebarProvider } from "@/components/ui/sidebar";
import { AuthType } from "@/gen/elara/auth/v1/auth_pb";
import { MeResponseSchema } from "@/gen/elara/profile/v1/profile_service_pb";
import { TestProviders } from "@/test/test-utils";
import { AppSidebar } from "./app-sidebar";

const adminUser = create(MeResponseSchema, {
	name: "Admin",
	email: "admin@elara.local",
	permissions: [],
	passwordChangeRequired: false,
	picture: "",
});

const mockAdminContext: AuthContextType = {
	state: {
		status: "authenticated",
		authType: AuthType.BASIC,
		user: adminUser,
		ability: buildAbility([{ object: 99, action: 99, domain: "*" } as any]),
	},
	logout: async () => {},
};

const mockUserContext: AuthContextType = {
	state: {
		status: "authenticated",
		authType: AuthType.BASIC,
		user: create(MeResponseSchema, {
			name: "User",
			email: "user@elara.local",
			permissions: [],
			passwordChangeRequired: false,
			picture: "",
		}),
		ability: buildAbility([
			{ object: 1, action: 1, domain: "*" } as any, // NAMESPACE READ (required by uiVisibility for Tokens section)
			{ object: 5, action: 1, domain: "*" } as any, // TOKEN READ
			{ object: 5, action: 2, domain: "*" } as any, // TOKEN WRITE
		]),
	},
	logout: async () => {},
};

const mockOidcAdminContext: AuthContextType = {
	state: {
		status: "authenticated",
		authType: AuthType.OIDC,
		user: adminUser,
		ability: buildAbility([{ object: 99, action: 99, domain: "*" } as any]),
	},
	logout: async () => {},
};

const mockNoneContext: AuthContextType = {
	state: {
		status: "authenticated",
		authType: AuthType.NONE,
		user: adminUser,
		ability: buildAbility([{ object: 99, action: 99, domain: "*" } as any]),
	},
	logout: async () => {},
};

describe("AppSidebar", () => {
	it("renders navigation items for admin", () => {
		render(
			<TestProviders authContext={mockAdminContext}>
				<SidebarProvider>
					<AppSidebar />
				</SidebarProvider>
			</TestProviders>,
		);

		expect(screen.getByText("Dashboard")).toBeInTheDocument();
		expect(screen.getByText("Configs")).toBeInTheDocument();
		expect(screen.getByText("Namespaces")).toBeInTheDocument();
		expect(screen.getByText("Clients")).toBeInTheDocument();
		expect(screen.getByText("Webhooks")).toBeInTheDocument();
		expect(screen.getByText("Tokens")).toBeInTheDocument();
	});

	it("shows Tokens but filters Webhooks for non-admin user", () => {
		render(
			<TestProviders authContext={mockUserContext}>
				<SidebarProvider>
					<AppSidebar />
				</SidebarProvider>
			</TestProviders>,
		);

		expect(screen.getByText("Dashboard")).toBeInTheDocument();
		expect(screen.queryByText("Webhooks")).not.toBeInTheDocument();
		expect(screen.getByText("Tokens")).toBeInTheDocument();
	});

	it("shows Webhooks for user with canViewWebhooks permission", () => {
		const mockUserWithWebhooksContext: AuthContextType = {
			state: {
				status: "authenticated",
				authType: AuthType.BASIC,
				user: create(MeResponseSchema, {
					name: "User",
					email: "user@elara.local",
					permissions: [],
					passwordChangeRequired: false,
					picture: "",
				}),
				ability: buildAbility([
					{ object: 6, action: 1, domain: "*" } as any, // WEBHOOK READ
				]),
			},
			logout: async () => {},
		};
		render(
			<TestProviders authContext={mockUserWithWebhooksContext}>
				<SidebarProvider>
					<AppSidebar />
				</SidebarProvider>
			</TestProviders>,
		);

		expect(screen.getByText("Webhooks")).toBeInTheDocument();
	});

	it("shows Administration group for BASIC and OIDC admin", () => {
		// BASIC Admin
		const { rerender } = render(
			<TestProviders authContext={mockAdminContext}>
				<SidebarProvider>
					<AppSidebar />
				</SidebarProvider>
			</TestProviders>,
		);

		expect(screen.getByText("Administration")).toBeInTheDocument();
		expect(screen.getByText("Users")).toBeInTheDocument();

		// OIDC Admin
		rerender(
			<TestProviders authContext={mockOidcAdminContext}>
				<SidebarProvider>
					<AppSidebar />
				</SidebarProvider>
			</TestProviders>,
		);
		expect(screen.getByText("Administration")).toBeInTheDocument();
	});

	it("hides Administration group for authType NONE", () => {
		render(
			<TestProviders authContext={mockNoneContext}>
				<SidebarProvider>
					<AppSidebar />
				</SidebarProvider>
			</TestProviders>,
		);

		expect(screen.queryByText("Administration")).not.toBeInTheDocument();
	});

	it("shows Webhooks and Tokens when authType is NONE", () => {
		render(
			<TestProviders authContext={mockNoneContext}>
				<SidebarProvider>
					<AppSidebar />
				</SidebarProvider>
			</TestProviders>,
		);

		expect(screen.getByText("Webhooks")).toBeInTheDocument();
		expect(screen.getByText("Tokens")).toBeInTheDocument();
	});

	it("highlights active item based on pathname", () => {
		render(
			<TestProviders
				initialEntries={["/namespaces"]}
				authContext={mockAdminContext}
			>
				<SidebarProvider>
					<AppSidebar />
				</SidebarProvider>
			</TestProviders>,
		);

		const namespacesLink = screen.getByRole("link", { name: "Namespaces" });
		expect(namespacesLink).toHaveAttribute("data-active");

		const dashboardLink = screen.getByRole("link", { name: "Dashboard" });
		expect(dashboardLink).not.toHaveAttribute("data-active");
	});

	it("adjusts Configs link when namespace is present", () => {
		render(
			<TestProviders
				initialEntries={["/browse/my-ns"]}
				authContext={mockAdminContext}
			>
				<SidebarProvider>
					<Routes>
						<Route path="/browse/:namespace" element={<AppSidebar />} />
					</Routes>
				</SidebarProvider>
			</TestProviders>,
		);

		const configsLink = screen.getByRole("link", { name: "Configs" });
		expect(configsLink).toHaveAttribute("href", "/browse/my-ns");
	});
});

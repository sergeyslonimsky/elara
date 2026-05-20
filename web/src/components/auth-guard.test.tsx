import { create } from "@bufbuild/protobuf";
import { Code, ConnectError } from "@connectrpc/connect";
import { render, screen } from "@testing-library/react";
import { Route, Routes } from "react-router";
import { describe, expect, it, vi } from "vitest";
import { buildAbility } from "@/auth/ability";
import type { AuthContextType } from "@/components/auth-provider";
import { AuthType } from "@/gen/elara/auth/v1/auth_pb";
import { MeResponseSchema } from "@/gen/elara/profile/v1/profile_service_pb";
import { TestProviders } from "@/test/test-utils";
import { AuthGuard } from "./auth-guard";

const authenticatedUser = create(MeResponseSchema, {
	name: "Admin",
	email: "admin@elara.local",
	permissions: [],
	passwordChangeRequired: false,
	picture: "",
});

function TestApp() {
	return (
		<Routes>
			<Route element={<AuthGuard />}>
				<Route path="/login" element={<div>Login Page</div>} />
				<Route
					path="/change-password"
					element={<div>Change Password Page</div>}
				/>
				<Route path="/auth/callback" element={<div>Callback Page</div>} />
				<Route path="/" element={<div>Dashboard</div>} />
				<Route path="*" element={<div>Not Found</div>} />
			</Route>
		</Routes>
	);
}

describe("AuthGuard", () => {
	it("shows loader when state is loading", () => {
		const authContext: AuthContextType = {
			state: { status: "loading" },
			logout: vi.fn(),
		};
		render(
			<TestProviders initialEntries={["/"]} authContext={authContext}>
				<TestApp />
			</TestProviders>,
		);
		expect(screen.getByRole("status", { name: "Loading" })).toBeInTheDocument();
	});

	it("shows error UI with retry button when state is error", () => {
		const authContext: AuthContextType = {
			state: {
				status: "error",
				error: new ConnectError("Server unavailable", Code.Unavailable),
			},
			logout: vi.fn(),
		};
		render(
			<TestProviders initialEntries={["/"]} authContext={authContext}>
				<TestApp />
			</TestProviders>,
		);
		expect(screen.getByText(/Server unavailable/)).toBeInTheDocument();
		expect(screen.getByRole("button", { name: "Retry" })).toBeInTheDocument();
	});

	it("allows anonymous access to public paths like /login", () => {
		const authContext: AuthContextType = {
			state: { status: "anonymous", authType: AuthType.BASIC },
			logout: vi.fn(),
		};
		render(
			<TestProviders initialEntries={["/login"]} authContext={authContext}>
				<TestApp />
			</TestProviders>,
		);
		expect(screen.getByText("Login Page")).toBeInTheDocument();
	});

	it("redirects anonymous user from protected route to /login", () => {
		const authContext: AuthContextType = {
			state: { status: "anonymous", authType: AuthType.BASIC },
			logout: vi.fn(),
		};
		render(
			<TestProviders initialEntries={["/"]} authContext={authContext}>
				<TestApp />
			</TestProviders>,
		);
		expect(screen.getByText("Login Page")).toBeInTheDocument();
		expect(screen.queryByText("Dashboard")).not.toBeInTheDocument();
	});

	it("redirects authenticated user from /login to /", () => {
		const authContext: AuthContextType = {
			state: {
				status: "authenticated",
				authType: AuthType.BASIC,
				user: authenticatedUser,
				ability: buildAbility([]),
			},
			logout: vi.fn(),
		};
		render(
			<TestProviders initialEntries={["/login"]} authContext={authContext}>
				<TestApp />
			</TestProviders>,
		);
		expect(screen.getByText("Dashboard")).toBeInTheDocument();
		expect(screen.queryByText("Login Page")).not.toBeInTheDocument();
	});

	it("redirects authenticated user with passwordChangeRequired to /change-password", () => {
		const userWithPasswordChange = create(MeResponseSchema, {
			...authenticatedUser,
			passwordChangeRequired: true,
		});
		const authContext: AuthContextType = {
			state: {
				status: "authenticated",
				authType: AuthType.BASIC,
				user: userWithPasswordChange,
				ability: buildAbility([]),
			},
			logout: vi.fn(),
		};
		render(
			<TestProviders initialEntries={["/"]} authContext={authContext}>
				<TestApp />
			</TestProviders>,
		);
		expect(screen.getByText("Change Password Page")).toBeInTheDocument();
		expect(screen.queryByText("Dashboard")).not.toBeInTheDocument();
	});

	it("allows authenticated user to access protected routes", () => {
		const authContext: AuthContextType = {
			state: {
				status: "authenticated",
				authType: AuthType.BASIC,
				user: authenticatedUser,
				ability: buildAbility([]),
			},
			logout: vi.fn(),
		};
		render(
			<TestProviders initialEntries={["/"]} authContext={authContext}>
				<TestApp />
			</TestProviders>,
		);
		expect(screen.getByText("Dashboard")).toBeInTheDocument();
	});

	it("redirects authenticated user with passwordChangeRequired from /login to /change-password", () => {
		const userWithPasswordChange = create(MeResponseSchema, {
			...authenticatedUser,
			passwordChangeRequired: true,
		});
		const authContext: AuthContextType = {
			state: {
				status: "authenticated",
				authType: AuthType.BASIC,
				user: userWithPasswordChange,
				ability: buildAbility([]),
			},
			logout: vi.fn(),
		};
		render(
			<TestProviders initialEntries={["/login"]} authContext={authContext}>
				<TestApp />
			</TestProviders>,
		);
		expect(screen.getByText("Change Password Page")).toBeInTheDocument();
		expect(screen.queryByText("Login Page")).not.toBeInTheDocument();
	});

	it("allows anonymous access to /auth/callback", () => {
		const authContext: AuthContextType = {
			state: { status: "anonymous", authType: AuthType.OIDC },
			logout: vi.fn(),
		};
		render(
			<TestProviders
				initialEntries={["/auth/callback"]}
				authContext={authContext}
			>
				<TestApp />
			</TestProviders>,
		);
		expect(screen.getByText("Callback Page")).toBeInTheDocument();
	});
});

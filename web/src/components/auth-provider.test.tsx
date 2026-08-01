import { create } from "@bufbuild/protobuf";
import { Code, ConnectError, createRouterTransport } from "@connectrpc/connect";
import { TransportProvider } from "@connectrpc/connect-query";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { describe, expect, it } from "vitest";
import { AuthProvider, useAuth } from "@/components/auth-provider";
import { AuthType } from "@/gen/elara/auth/v1/auth_pb";
import { AuthService } from "@/gen/elara/auth/v1/auth_service_pb";
import {
	CapabilitiesService,
	GetCapabilitiesResponseSchema,
} from "@/gen/elara/capabilities/v1/capabilities_service_pb";
import {
	MeResponseSchema,
	ProfileService,
} from "@/gen/elara/profile/v1/profile_service_pb";

const authenticatedUser = create(MeResponseSchema, {
	name: "Admin",
	email: "admin@elara.local",
	permissions: [],
	passwordChangeRequired: false,
	picture: "",
});

const allCapabilitiesEnabled = create(GetCapabilitiesResponseSchema, {
	etcdTokenAuthEnabled: true,
	userManagementEnabled: true,
});

function createDeferred<T>() {
	let resolve!: (value: T) => void;
	let reject!: (reason?: unknown) => void;
	const promise = new Promise<T>((res, rej) => {
		resolve = res;
		reject = rej;
	});
	return { promise, resolve, reject };
}

/** Renders a small probe component reading useAuth().state through the real AuthProviderInner. */
function AuthStateProbe() {
	const { state } = useAuth();

	switch (state.status) {
		case "loading":
			return <div>status:loading</div>;
		case "error":
			return <div>status:error - {state.error.message}</div>;
		case "anonymous":
			return <div>status:anonymous</div>;
		case "authenticated":
			return (
				<div>
					<div>status:authenticated</div>
					<div data-testid="capabilities">
						{JSON.stringify({
							etcdTokenAuthEnabled: state.capabilities.etcdTokenAuthEnabled,
							userManagementEnabled: state.capabilities.userManagementEnabled,
						})}
					</div>
				</div>
			);
		default:
			return null;
	}
}

function renderWithTransport(
	transport: ReturnType<typeof createRouterTransport>,
) {
	const queryClient = new QueryClient({
		defaultOptions: { queries: { retry: false, staleTime: 0 } },
	});

	function Wrapper({ children }: { children: ReactNode }) {
		return (
			<TransportProvider transport={transport}>
				<QueryClientProvider client={queryClient}>
					<AuthProvider>{children}</AuthProvider>
				</QueryClientProvider>
			</TransportProvider>
		);
	}

	return render(<AuthStateProbe />, { wrapper: Wrapper });
}

describe("AuthProviderInner (real fetch flow)", () => {
	it("reaches authenticated status once getAuthInfo, me, and getCapabilities all resolve", async () => {
		const transport = createRouterTransport(({ rpc }) => {
			rpc(AuthService.method.getAuthInfo, () => ({ authType: AuthType.NONE }));
			rpc(ProfileService.method.me, () => authenticatedUser);
			rpc(
				CapabilitiesService.method.getCapabilities,
				() => allCapabilitiesEnabled,
			);
		});

		renderWithTransport(transport);

		await screen.findByText("status:authenticated");
		expect(screen.getByTestId("capabilities")).toHaveTextContent(
			JSON.stringify({
				etcdTokenAuthEnabled: true,
				userManagementEnabled: true,
			}),
		);
	});

	it("moves to error status when getCapabilities fails", async () => {
		const transport = createRouterTransport(({ rpc }) => {
			rpc(AuthService.method.getAuthInfo, () => ({ authType: AuthType.NONE }));
			rpc(ProfileService.method.me, () => authenticatedUser);
			rpc(CapabilitiesService.method.getCapabilities, () => {
				throw new ConnectError("capabilities unavailable", Code.Unavailable);
			});
		});

		renderWithTransport(transport);

		await screen.findByText(/status:error.*capabilities unavailable/);
		expect(screen.queryByText("status:authenticated")).not.toBeInTheDocument();
	});

	it("stays in loading status after me resolves but before getCapabilities resolves", async () => {
		const deferredCapabilities =
			createDeferred<typeof allCapabilitiesEnabled>();

		const transport = createRouterTransport(({ rpc }) => {
			rpc(AuthService.method.getAuthInfo, () => ({ authType: AuthType.NONE }));
			rpc(ProfileService.method.me, () => authenticatedUser);
			rpc(
				CapabilitiesService.method.getCapabilities,
				() => deferredCapabilities.promise,
			);
		});

		renderWithTransport(transport);

		// me has resolved (authType known + user present) but capabilities has not —
		// the provider must not flash "authenticated" without capabilities data.
		await waitFor(() => {
			expect(screen.getByText("status:loading")).toBeInTheDocument();
		});
		expect(screen.queryByText("status:authenticated")).not.toBeInTheDocument();

		deferredCapabilities.resolve(allCapabilitiesEnabled);

		await screen.findByText("status:authenticated");
	});
});

import { create, type MessageInitShape } from "@bufbuild/protobuf";
import { createRouterTransport } from "@connectrpc/connect";
import { TransportProvider } from "@connectrpc/connect-query";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { ReactNode } from "react";
import { MemoryRouter } from "react-router";
import { vi } from "vitest";
import type { AppAbility } from "@/auth/ability";
import { AbilityProvider } from "@/auth/ability-context";
import { type AuthContextType, AuthProvider } from "@/components/auth-provider";
import { ThemeProvider } from "@/components/theme-provider";
import { Toaster } from "@/components/ui/sonner";
import { TooltipProvider } from "@/components/ui/tooltip";
import { AuthType } from "@/gen/elara/auth/v1/auth_pb";
import { MeResponseSchema } from "@/gen/elara/profile/v1/profile_service_pb";

// In-process transport — no network calls during tests.
const transport = createRouterTransport(() => {});

const testQueryClient = new QueryClient({
	defaultOptions: {
		queries: {
			retry: false,
			staleTime: 0,
		},
	},
});

// authenticatedContext builds an AuthContextType for an authenticated caller
// with the given ability. Pass it to <TestProviders authContext={...}> so the
// component under test reads ability through the real AuthProvider →
// AbilityProvider → AbilityContext chain, instead of mocking useAuth/useAbility.
export function authenticatedContext(
	ability: AppAbility,
	opts?: {
		authType?: AuthType;
		user?: MessageInitShape<typeof MeResponseSchema>;
		logout?: () => Promise<void>;
	},
): AuthContextType {
	return {
		state: {
			status: "authenticated",
			authType: opts?.authType ?? AuthType.UNSPECIFIED,
			ability,
			user: create(MeResponseSchema, {
				email: "admin@example.com",
				name: "Admin",
				...opts?.user,
			}),
		},
		logout: opts?.logout ?? vi.fn(),
	};
}

export function TestProviders({
	children,
	initialEntries,
	authContext,
}: {
	children: ReactNode;
	initialEntries?: string[];
	authContext?: AuthContextType;
}) {
	const content = authContext ? (
		<AuthProvider initialValue={authContext}>
			<AbilityProvider>{children}</AbilityProvider>
		</AuthProvider>
	) : (
		<AuthProvider>
			<AbilityProvider>{children}</AbilityProvider>
		</AuthProvider>
	);

	return (
		<MemoryRouter initialEntries={initialEntries}>
			<TransportProvider transport={transport}>
				<QueryClientProvider client={testQueryClient}>
					<ThemeProvider defaultTheme="system" storageKey="elara-theme">
						<TooltipProvider>
							{content}
							<Toaster richColors />
						</TooltipProvider>
					</ThemeProvider>
				</QueryClientProvider>
			</TransportProvider>
		</MemoryRouter>
	);
}

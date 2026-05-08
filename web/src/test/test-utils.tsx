import { createRouterTransport } from "@connectrpc/connect";
import { TransportProvider } from "@connectrpc/connect-query";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { ReactNode } from "react";
import { MemoryRouter } from "react-router";
import { type AuthContextType, AuthProvider } from "@/components/auth-provider";
import { ThemeProvider } from "@/components/theme-provider";
import { Toaster } from "@/components/ui/sonner";
import { TooltipProvider } from "@/components/ui/tooltip";

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
		<AuthProvider initialValue={authContext}>{children}</AuthProvider>
	) : (
		<AuthProvider>{children}</AuthProvider>
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

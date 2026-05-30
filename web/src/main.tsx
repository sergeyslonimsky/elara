// @ts-expect-error font CSS import
import "@fontsource-variable/public-sans";
// @ts-expect-error font CSS import
import "@fontsource-variable/geist";
import "./index.css";

import { Code, ConnectError } from "@connectrpc/connect";
import {
	createConnectQueryKey,
	TransportProvider,
} from "@connectrpc/connect-query";
import {
	MutationCache,
	QueryCache,
	QueryClient,
	QueryClientProvider,
} from "@tanstack/react-query";
import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { BrowserRouter } from "react-router";
import { AbilityProvider } from "@/auth/ability-context";
import { AuthProvider } from "@/components/auth-provider";
import { ThemeProvider } from "@/components/theme-provider";
import { Toaster } from "@/components/ui/sonner";
import { TooltipProvider } from "@/components/ui/tooltip";
import { me } from "@/gen/elara/profile/v1/profile_service-ProfileService_connectquery";
import { createAuthAwareTransport } from "@/lib/transport";
import App from "./App";

function isPermissionDenied(error: unknown): boolean {
	if (error instanceof ConnectError) {
		return error.code === Code.PermissionDenied;
	}
	return false;
}

const queryClient = new QueryClient({
	queryCache: new QueryCache({
		onError: (error) => {
			if (isPermissionDenied(error)) {
				void queryClient.refetchQueries({
					queryKey: createConnectQueryKey({
						schema: me,
						cardinality: undefined,
					}),
				});
			}
		},
	}),
	mutationCache: new MutationCache({
		onError: (error) => {
			if (isPermissionDenied(error)) {
				void queryClient.refetchQueries({
					queryKey: createConnectQueryKey({
						schema: me,
						cardinality: undefined,
					}),
				});
			}
		},
	}),
	defaultOptions: {
		queries: {
			retry: (failureCount, error) =>
				failureCount < 3 && !isPermissionDenied(error),
			staleTime: 30_000,
		},
	},
});

const transport = createAuthAwareTransport(queryClient);

// biome-ignore lint/style/noNonNullAssertion: root element guaranteed by index.html
createRoot(document.getElementById("root")!).render(
	<StrictMode>
		<TransportProvider transport={transport}>
			<QueryClientProvider client={queryClient}>
				<BrowserRouter>
					<AuthProvider>
						<AbilityProvider>
							<ThemeProvider defaultTheme="system" storageKey="elara-theme">
								<TooltipProvider>
									<App />
									<Toaster richColors />
								</TooltipProvider>
							</ThemeProvider>
						</AbilityProvider>
					</AuthProvider>
				</BrowserRouter>
			</QueryClientProvider>
		</TransportProvider>
	</StrictMode>,
);

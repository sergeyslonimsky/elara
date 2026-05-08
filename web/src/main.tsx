// @ts-expect-error font CSS import
import "@fontsource-variable/public-sans";
// @ts-expect-error font CSS import
import "@fontsource-variable/geist";
import "./index.css";

import { Code, ConnectError } from "@connectrpc/connect";
import { TransportProvider } from "@connectrpc/connect-query";
import { createConnectTransport } from "@connectrpc/connect-web";
import {
	QueryCache,
	QueryClient,
	QueryClientProvider,
} from "@tanstack/react-query";
import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { BrowserRouter } from "react-router";
import { AuthProvider } from "@/components/auth-provider";
import { ThemeProvider } from "@/components/theme-provider";
import { Toaster } from "@/components/ui/sonner";
import { TooltipProvider } from "@/components/ui/tooltip";
import App from "./App";

const transport = createConnectTransport({
	baseUrl: window.location.origin,
	fetch: (input, init) =>
		globalThis.fetch(input, { ...init, credentials: "include" }),
});

const queryClient = new QueryClient({
	queryCache: new QueryCache({
		onError: (error) => {
			if (
				error instanceof ConnectError &&
				error.code === Code.PermissionDenied &&
				error.message.toLowerCase().includes("password change required")
			) {
				window.location.href = "/change-password";
			}
		},
	}),
	defaultOptions: {
		queries: {
			retry: 1,
			staleTime: 30_000,
		},
	},
});

// biome-ignore lint/style/noNonNullAssertion: root element guaranteed by index.html
createRoot(document.getElementById("root")!).render(
	<StrictMode>
		<TransportProvider transport={transport}>
			<QueryClientProvider client={queryClient}>
				<BrowserRouter>
					<AuthProvider>
						<ThemeProvider defaultTheme="system" storageKey="elara-theme">
							<TooltipProvider>
								<App />
								<Toaster richColors />
							</TooltipProvider>
						</ThemeProvider>
					</AuthProvider>
				</BrowserRouter>
			</QueryClientProvider>
		</TransportProvider>
	</StrictMode>,
);

// @ts-expect-error font CSS import
import "@fontsource-variable/public-sans";
// @ts-expect-error font CSS import
import "@fontsource-variable/geist";
import "./index.css";

import { TransportProvider } from "@connectrpc/connect-query";
import { createConnectTransport } from "@connectrpc/connect-web";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { BrowserRouter } from "react-router";
import { ThemeProvider } from "@/components/theme-provider";
import { Toaster } from "@/components/ui/sonner";
import { TooltipProvider } from "@/components/ui/tooltip";
import App from "./App";

const transport = createConnectTransport({
	baseUrl: window.location.origin,
});

const queryClient = new QueryClient({
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
					<ThemeProvider defaultTheme="system" storageKey="elara-theme">
						<TooltipProvider>
							<App />
							<Toaster richColors />
						</TooltipProvider>
					</ThemeProvider>
				</BrowserRouter>
			</QueryClientProvider>
		</TransportProvider>
	</StrictMode>,
);

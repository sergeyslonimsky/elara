import { TransportProvider } from "@connectrpc/connect-query";
import { createConnectTransport } from "@connectrpc/connect-web";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { ReactNode } from "react";
import { ThemeProvider } from "@/components/theme-provider";
import { Toaster } from "@/components/ui/sonner";
import { TooltipProvider } from "@/components/ui/tooltip";

const transport = createConnectTransport({
	baseUrl: "http://localhost:8080",
});

const testQueryClient = new QueryClient({
	defaultOptions: {
		queries: {
			retry: false,
			staleTime: 0,
		},
	},
});

export function TestProviders({ children }: { children: ReactNode }) {
	return (
		<TransportProvider transport={transport}>
			<QueryClientProvider client={testQueryClient}>
				<ThemeProvider defaultTheme="system" storageKey="elara-theme">
					<TooltipProvider>
						{children}
						<Toaster richColors />
					</TooltipProvider>
				</ThemeProvider>
			</QueryClientProvider>
		</TransportProvider>
	);
}

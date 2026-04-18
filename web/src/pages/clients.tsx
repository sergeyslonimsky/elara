import { useQuery } from "@connectrpc/connect-query";
import type { SortingState } from "@tanstack/react-table";
import { Network, RefreshCw } from "lucide-react";
import { useMemo, useState } from "react";
import { AppHeader } from "@/components/app-header";
import { ClientsTable } from "@/components/clients-table";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import {
	Empty,
	EmptyContent,
	EmptyDescription,
	EmptyHeader,
	EmptyMedia,
	EmptyTitle,
} from "@/components/ui/empty";
import { Input } from "@/components/ui/input";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import type { Client } from "@/gen/elara/clients/v1/clients_pb";
import {
	listActiveClients,
	listHistoricalConnections,
} from "@/gen/elara/clients/v1/clients_service-ClientsService_connectquery";
import { cn } from "@/lib/utils";

function filterClients(
	clients: Client[] | undefined,
	search: string,
): Client[] {
	if (!clients) return [];
	const q = search.trim().toLowerCase();
	if (!q) return clients;
	return clients.filter(
		(c) =>
			c.clientName.toLowerCase().includes(q) ||
			c.peerAddress.toLowerCase().includes(q) ||
			c.k8sPod.toLowerCase().includes(q) ||
			c.userAgent.toLowerCase().includes(q),
	);
}

export function ClientsPage() {
	const [tab, setTab] = useState<"active" | "history">("active");
	const [search, setSearch] = useState("");
	const [activeSorting, setActiveSorting] = useState<SortingState>([
		{ id: "lastActivityAt", desc: true },
	]);
	const [historySorting, setHistorySorting] = useState<SortingState>([
		{ id: "disconnectedAt", desc: true },
	]);

	const activeQ = useQuery(listActiveClients, {});
	const historyQ = useQuery(listHistoricalConnections, { limit: 200 });

	const activeClients = useMemo(
		() => filterClients(activeQ.data?.clients, search),
		[activeQ.data, search],
	);
	const historyClients = useMemo(
		() => filterClients(historyQ.data?.clients, search),
		[historyQ.data, search],
	);

	// Refetch via the query handles directly. Don't rely on guessing the
	// ConnectRPC-generated query key shape — it's an internal detail.
	const refresh = () => {
		void activeQ.refetch();
		void historyQ.refetch();
	};

	const isRefreshing = activeQ.isFetching || historyQ.isFetching;

	return (
		<>
			<AppHeader />
			<div className="flex flex-1 flex-col gap-4 p-4 pt-0">
				<div className="mt-4 flex items-center gap-3">
					<h1 className="font-semibold text-xl">Clients</h1>
					<Button
						variant="outline"
						size="icon"
						onClick={refresh}
						aria-label="Refresh"
					>
						<RefreshCw
							className={cn("h-4 w-4", isRefreshing && "animate-spin")}
						/>
					</Button>
					<Input
						placeholder="Search by name / pod / peer..."
						value={search}
						onChange={(e) => setSearch(e.target.value)}
						className="ml-auto w-72"
					/>
				</div>

				<Tabs
					value={tab}
					onValueChange={(v) => setTab(v as "active" | "history")}
				>
					<TabsList>
						<TabsTrigger value="active">
							Active{" "}
							{activeQ.data && (
								<span className="ml-1 text-muted-foreground">
									({activeQ.data.clients.length})
								</span>
							)}
						</TabsTrigger>
						<TabsTrigger value="history">History</TabsTrigger>
					</TabsList>

					<TabsContent value="active" className="space-y-3">
						<Card className="rounded-xl">
							<CardContent className="pt-4">
								<ClientsTable
									clients={activeClients}
									isLoading={activeQ.isLoading}
									mode="active"
									sorting={activeSorting}
									onSortingChange={setActiveSorting}
									emptySlot={
										<Empty>
											<EmptyHeader>
												<EmptyMedia variant="icon">
													<Network />
												</EmptyMedia>
												<EmptyTitle>No connected clients</EmptyTitle>
												<EmptyDescription>
													Try connecting an etcd client to{" "}
													<code className="rounded bg-muted px-1 py-0.5 text-xs">
														localhost:2379
													</code>
													. Set the{" "}
													<code className="rounded bg-muted px-1 py-0.5 text-xs">
														x-client-name
													</code>{" "}
													metadata header so the client shows up with a name.
												</EmptyDescription>
											</EmptyHeader>
											<EmptyContent />
										</Empty>
									}
								/>
							</CardContent>
						</Card>
					</TabsContent>

					<TabsContent value="history" className="space-y-3">
						<Card className="rounded-xl">
							<CardContent className="pt-4">
								<ClientsTable
									clients={historyClients}
									isLoading={historyQ.isLoading}
									mode="history"
									sorting={historySorting}
									onSortingChange={setHistorySorting}
									emptySlot={
										<Empty>
											<EmptyHeader>
												<EmptyMedia variant="icon">
													<Network />
												</EmptyMedia>
												<EmptyTitle>No past connections</EmptyTitle>
												<EmptyDescription>
													Closed connections will appear here.
												</EmptyDescription>
											</EmptyHeader>
											<EmptyContent />
										</Empty>
									}
								/>
							</CardContent>
						</Card>
					</TabsContent>
				</Tabs>
			</div>
		</>
	);
}

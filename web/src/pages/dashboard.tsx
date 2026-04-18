import { useQuery } from "@connectrpc/connect-query";
import { FolderOpen, RefreshCw, Zap } from "lucide-react";
import { Link } from "react-router";
import { AppHeader } from "@/components/app-header";
import { ErrorCard } from "@/components/error-card";
import { KpiCard } from "@/components/kpi-card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import {
	Table,
	TableBody,
	TableCell,
	TableHead,
	TableHeader,
	TableRow,
} from "@/components/ui/table";
import { EventType } from "@/gen/elara/config/v1/config_pb";
import {
	getStats,
	listActivity,
} from "@/gen/elara/dashboard/v1/dashboard_service-DashboardService_connectquery";
import { listNamespaces } from "@/gen/elara/namespace/v1/namespace_service-NamespaceService_connectquery";
import { timeAgo, tsToMs } from "@/lib/time";
import { cn } from "@/lib/utils";

const DEFAULT_ACTIVITIES_COUNT = 20;

// Stable keys for loading skeletons. Using array-index keys triggers React
// warnings about reordering/state; for static-length skeletons we use unique
// string IDs instead.
const ACTIVITY_SKELETON_KEYS = ["a1", "a2", "a3", "a4", "a5", "a6"];
const NAMESPACE_SKELETON_KEYS = ["n1", "n2", "n3", "n4"];

export function DashboardPage() {
	const statsQ = useQuery(getStats, {}, { refetchInterval: 30_000 });
	const activityQ = useQuery(
		listActivity,
		{ limit: DEFAULT_ACTIVITIES_COUNT },
		{ refetchInterval: 30_000 },
	);
	const namespacesQ = useQuery(listNamespaces, {});

	const isRefreshing = statsQ.isFetching || activityQ.isFetching;

	const refresh = () => {
		void statsQ.refetch();
		void activityQ.refetch();
		void namespacesQ.refetch();
	};

	return (
		<>
			<AppHeader />
			<div className="flex flex-1 flex-col gap-6 p-4 pt-0">
				<div className="mt-4 flex items-center gap-3">
					<h1 className="font-semibold text-xl">Dashboard</h1>
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
				</div>

				{statsQ.error && <ErrorCard message={statsQ.error.message} />}
				{activityQ.error && <ErrorCard message={activityQ.error.message} />}

				{/* KPI row */}
				<div className="grid grid-cols-2 gap-4 md:grid-cols-4">
					<KpiCard
						label="Namespaces"
						value={statsQ.data?.namespaceCount ?? "—"}
					/>
					<KpiCard label="Configs" value={statsQ.data?.configCount ?? "—"} />
					<KpiCard
						label="Active Clients"
						value={statsQ.data?.activeClientCount ?? "—"}
						accentClass="text-emerald-500"
					/>
					<KpiCard
						label="Global Revision"
						value={
							statsQ.data ? statsQ.data.globalRevision.toLocaleString() : "—"
						}
						accentClass="text-blue-500"
					/>
				</div>

				<div className="grid gap-4 lg:grid-cols-3">
					{/* Recent Activity (wider) */}
					<Card className="rounded-xl lg:col-span-2">
						<CardHeader className="pb-3">
							<CardTitle className="text-base">
								Last {DEFAULT_ACTIVITIES_COUNT} Changes
							</CardTitle>
						</CardHeader>
						<CardContent className="p-0">
							<Table>
								<TableHeader>
									<TableRow>
										<TableHead className="w-28">Type</TableHead>
										<TableHead>Namespace</TableHead>
										<TableHead>Path</TableHead>
										<TableHead className="w-28 text-right">When</TableHead>
									</TableRow>
								</TableHeader>
								<TableBody>
									{activityQ.isLoading &&
										ACTIVITY_SKELETON_KEYS.map((key) => (
											<TableRow key={key}>
												<TableCell colSpan={4}>
													<div className="h-4 animate-pulse rounded bg-muted" />
												</TableCell>
											</TableRow>
										))}
									{activityQ.data?.entries.length === 0 && (
										<TableRow>
											<TableCell
												colSpan={4}
												className="py-8 text-center text-muted-foreground text-sm"
											>
												No activity yet
											</TableCell>
										</TableRow>
									)}
									{activityQ.data?.entries.map((entry) => (
										<TableRow key={entry.revision.toString()}>
											<TableCell>
												<EventTypeBadge type={entry.eventType} />
											</TableCell>
											<TableCell className="font-mono text-xs">
												{entry.namespace || (
													<span className="text-muted-foreground">—</span>
												)}
											</TableCell>
											<TableCell className="max-w-[240px] truncate font-mono text-xs">
												<Link
													to={`/config/${entry.namespace}${entry.path}`}
													className="hover:underline"
												>
													{entry.path}
												</Link>
											</TableCell>
											<TableCell className="text-right text-muted-foreground text-xs">
												{entry.timestamp
													? timeAgo(new Date(tsToMs(entry.timestamp)))
													: "—"}
											</TableCell>
										</TableRow>
									))}
								</TableBody>
							</Table>
						</CardContent>
					</Card>

					{/* Namespace overview */}
					<Card className="rounded-xl">
						<CardHeader className="pb-3">
							<CardTitle className="text-base">Namespaces</CardTitle>
						</CardHeader>
						<CardContent className="p-0">
							<Table>
								<TableHeader>
									<TableRow>
										<TableHead>Name</TableHead>
										<TableHead className="w-20 text-right">Configs</TableHead>
									</TableRow>
								</TableHeader>
								<TableBody>
									{namespacesQ.isLoading &&
										NAMESPACE_SKELETON_KEYS.map((key) => (
											<TableRow key={key}>
												<TableCell colSpan={2}>
													<div className="h-4 animate-pulse rounded bg-muted" />
												</TableCell>
											</TableRow>
										))}
									{namespacesQ.data?.namespaces.length === 0 && (
										<TableRow>
											<TableCell
												colSpan={2}
												className="py-8 text-center text-muted-foreground text-sm"
											>
												No namespaces
											</TableCell>
										</TableRow>
									)}
									{namespacesQ.data?.namespaces.map((ns) => (
										<TableRow key={ns.name}>
											<TableCell>
												<Link
													to={`/browse/${ns.name}`}
													className="flex items-center gap-2 hover:underline"
												>
													<FolderOpen className="h-3.5 w-3.5 text-muted-foreground" />
													<span className="font-medium text-sm">{ns.name}</span>
												</Link>
											</TableCell>
											<TableCell className="text-right text-muted-foreground text-sm tabular-nums">
												{ns.configCount}
											</TableCell>
										</TableRow>
									))}
								</TableBody>
							</Table>
						</CardContent>
					</Card>
				</div>
			</div>
		</>
	);
}

function EventTypeBadge({ type }: { type: EventType }) {
	switch (type) {
		case EventType.CREATED:
			return (
				<Badge className="bg-emerald-100 text-emerald-700 hover:bg-emerald-100 dark:bg-emerald-900/30 dark:text-emerald-400">
					<Zap className="mr-1 h-3 w-3" />
					Created
				</Badge>
			);
		case EventType.UPDATED:
			return (
				<Badge className="bg-blue-100 text-blue-700 hover:bg-blue-100 dark:bg-blue-900/30 dark:text-blue-400">
					Updated
				</Badge>
			);
		case EventType.DELETED:
			return (
				<Badge className="bg-red-100 text-red-700 hover:bg-red-100 dark:bg-red-900/30 dark:text-red-400">
					Deleted
				</Badge>
			);
		default:
			return <Badge variant="outline">Unknown</Badge>;
	}
}

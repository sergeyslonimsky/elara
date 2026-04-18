import { useQuery } from "@connectrpc/connect-query";
import {
	type ColumnDef,
	flexRender,
	getCoreRowModel,
	getSortedRowModel,
	type SortingState,
	useReactTable,
} from "@tanstack/react-table";
import {
	AlertTriangle,
	ArrowLeft,
	BarChart3,
	CheckCircle2,
	Eye,
	History as HistoryIcon,
} from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import { Link, useNavigate, useParams } from "react-router";
import {
	Cell,
	Pie,
	PieChart,
	Tooltip as RechartsTooltip,
	ResponsiveContainer,
} from "recharts";
import { AppHeader } from "@/components/app-header";
import { ErrorCard } from "@/components/error-card";
import { KpiCard } from "@/components/kpi-card";
import { SortableHeader } from "@/components/sortable-header";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import {
	Empty,
	EmptyDescription,
	EmptyHeader,
	EmptyMedia,
	EmptyTitle,
} from "@/components/ui/empty";
import { Skeleton } from "@/components/ui/skeleton";
import {
	Table,
	TableBody,
	TableCell,
	TableHead,
	TableHeader,
	TableRow,
} from "@/components/ui/table";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import type { Client, ClientEvent } from "@/gen/elara/clients/v1/clients_pb";
import {
	getClient,
	listClientSessions,
} from "@/gen/elara/clients/v1/clients_service-ClientsService_connectquery";
import { useHistory } from "@/hooks/use-history";
import { type StreamStatus, useWatchClient } from "@/hooks/use-watch-client";
import { totalRequests } from "@/lib/client";
import { formatRpcDuration, formatUptime, isLive } from "@/lib/duration";
import { classifyWatch, type WatchTarget } from "@/lib/etcd-key";
import { timeAgo, tsToMs } from "@/lib/time";

const DONUT_COLORS = [
	"hsl(217 91% 60%)",
	"hsl(142 71% 45%)",
	"hsl(38 92% 50%)",
	"hsl(280 65% 60%)",
	"hsl(0 84% 60%)",
	"hsl(240 5% 65%)", // "other"
];

export function ClientDetailPage() {
	const { id = "" } = useParams();

	// Initial unary fetch — instant first paint
	const initialQ = useQuery(getClient, { id }, { enabled: !!id, staleTime: 0 });

	// Live stream — overrides snapshot when connected
	const live = useWatchClient(id);

	const client = live.snapshot ?? initialQ.data?.client;
	const isActive = client ? !client.disconnectedAt : false;

	// Activity: live events prepended to the initial server-side ring buffer.
	const initialEvents = initialQ.data?.recentEvents ?? [];
	const liveEvents = live.events;
	const events = useMemo(() => {
		// liveEvents are newest-first; initialEvents are oldest-first
		// (server returns chronological). Merge so newest is on top.
		const initialReversed = [...initialEvents].reverse();
		// Avoid duplication: if liveEvents already contains anything,
		// trust live as the source of truth past the merge boundary.
		if (liveEvents.length === 0) return initialReversed;
		const initialMs = initialReversed[0]
			? tsToMs(initialReversed[0].timestamp)
			: 0;
		const newOnly = liveEvents.filter((e) => tsToMs(e.timestamp) > initialMs);
		return [...newOnly, ...initialReversed];
	}, [liveEvents, initialEvents]);

	// Tab title: prefix with (disconnected) when client is gone.
	useEffect(() => {
		if (!client) return;
		const base = client.clientName || `${client.peerAddress}` || "client";
		document.title = isActive
			? `${base} • Elara`
			: `(disconnected) ${base} • Elara`;
		return () => {
			document.title = "Elara";
		};
	}, [client, isActive]);

	if (initialQ.isLoading && !client) {
		return (
			<>
				<AppHeader />
				<div className="flex flex-1 flex-col gap-4 p-4 pt-0">
					<Skeleton className="mt-4 h-8 w-48" />
					<Skeleton className="h-32 w-full rounded-xl" />
					<Skeleton className="h-64 w-full rounded-xl" />
				</div>
			</>
		);
	}

	if (initialQ.error || !client) {
		return (
			<>
				<AppHeader />
				<div className="flex flex-1 flex-col gap-4 p-4 pt-0">
					<BackLink />
					<ErrorCard message={initialQ.error?.message ?? "Client not found"} />
				</div>
			</>
		);
	}

	return (
		<>
			<AppHeader />
			<div className="flex flex-1 flex-col gap-4 p-4 pt-0">
				<BackLink />
				<DetailHeader
					client={client}
					isActive={isActive}
					streamStatus={live.status}
				/>
				<KpiRow client={client} isActive={isActive} />
				<DetailTabs client={client} events={events} isActive={isActive} />
			</div>
		</>
	);
}

function BackLink() {
	return (
		<div className="mt-4">
			<Button variant="ghost" size="sm" render={<Link to="/clients" />}>
				<ArrowLeft className="mr-1 h-4 w-4" />
				Back to clients
			</Button>
		</div>
	);
}

function DetailHeader({
	client,
	isActive,
	streamStatus,
}: {
	client: Client;
	isActive: boolean;
	streamStatus: StreamStatus;
}) {
	const live = isLive(tsToMs(client.lastActivityAt));
	return (
		<Card className="rounded-xl">
			<CardContent className="space-y-2 pt-4">
				<div className="flex flex-wrap items-center gap-3">
					{isActive ? (
						<Badge
							variant="default"
							className={
								live
									? "bg-emerald-500 hover:bg-emerald-500"
									: "bg-muted-foreground hover:bg-muted-foreground"
							}
						>
							{live ? "● Active" : "● Idle"}
						</Badge>
					) : (
						<Badge variant="destructive">● Disconnected</Badge>
					)}
					<h1 className="font-semibold text-xl">
						{client.clientName || (
							<span className="text-muted-foreground italic">unknown</span>
						)}
					</h1>
					{client.clientVersion && (
						<Badge variant="outline">v{client.clientVersion}</Badge>
					)}
					{!isActive && client.disconnectedAt && (
						<span className="text-muted-foreground text-xs">
							disconnected {timeAgo(new Date(tsToMs(client.disconnectedAt)))}
						</span>
					)}
					{isActive && (
						<span className="ml-auto text-muted-foreground text-xs">
							stream: {streamStatus}
						</span>
					)}
				</div>
				<div className="flex flex-wrap gap-x-4 gap-y-1 text-muted-foreground text-xs">
					<span>
						peer <span className="font-mono">{client.peerAddress}</span>
					</span>
					{client.userAgent && <span>ua {client.userAgent}</span>}
					{client.k8sNamespace && (
						<span>
							ns <span className="font-mono">{client.k8sNamespace}</span>
						</span>
					)}
					{client.k8sPod && (
						<span>
							pod <span className="font-mono">{client.k8sPod}</span>
						</span>
					)}
					{client.k8sNode && (
						<span>
							node <span className="font-mono">{client.k8sNode}</span>
						</span>
					)}
					{client.instanceId && (
						<span>
							instance <span className="font-mono">{client.instanceId}</span>
						</span>
					)}
				</div>
			</CardContent>
		</Card>
	);
}

function KpiRow({ client, isActive }: { client: Client; isActive: boolean }) {
	const requests = totalRequests(client);
	const errors = Number(client.errorCount);
	const watches = client.activeWatches;
	const requestSeries = useHistory(requests);
	const errorSeries = useHistory(errors);
	const watchSeries = useHistory(watches);

	const connectedAtMs = tsToMs(client.connectedAt);
	const disconnectedAtMs = client.disconnectedAt
		? tsToMs(client.disconnectedAt)
		: 0;
	const uptime = formatUptime(
		connectedAtMs,
		isActive ? Date.now() : disconnectedAtMs,
	);

	const lastActivityMs = tsToMs(client.lastActivityAt);
	const lastActivity = lastActivityMs ? timeAgo(new Date(lastActivityMs)) : "—";

	return (
		<div className="grid grid-cols-2 gap-3 md:grid-cols-3 xl:grid-cols-5">
			<KpiCard
				label="Requests"
				value={requests.toLocaleString()}
				series={requestSeries}
				accentClass="text-blue-500"
			/>
			<KpiCard
				label="Errors"
				value={errors.toLocaleString()}
				series={errorSeries}
				accentClass={errors > 0 ? "text-destructive" : "text-muted-foreground"}
			/>
			<KpiCard
				label="Active watches"
				value={watches}
				series={watchSeries}
				accentClass="text-violet-500"
			/>
			<KpiCard
				label="Uptime"
				value={uptime}
				subtitle={
					connectedAtMs
						? `since ${new Date(connectedAtMs).toLocaleString()}`
						: undefined
				}
			/>
			<KpiCard
				label="Last activity"
				value={lastActivity}
				subtitle={isActive ? "live" : undefined}
			/>
		</div>
	);
}

function DetailTabs({
	client,
	events,
	isActive,
}: {
	client: Client;
	events: ClientEvent[];
	isActive: boolean;
}) {
	const totalErrors = Number(client.errorCount);

	return (
		<Tabs defaultValue="activity">
			<TabsList>
				<TabsTrigger value="activity">Activity</TabsTrigger>
				<TabsTrigger value="counters">Counters</TabsTrigger>
				<TabsTrigger value="errors">
					Errors{" "}
					{totalErrors > 0 && (
						<span className="ml-1 text-destructive">({totalErrors})</span>
					)}
				</TabsTrigger>
				<TabsTrigger value="watches">Watches</TabsTrigger>
				<TabsTrigger value="sessions">Sessions</TabsTrigger>
			</TabsList>

			<TabsContent value="activity">
				<Card className="rounded-xl">
					<CardContent className="pt-4">
						<ActivityTab events={events} isActive={isActive} />
					</CardContent>
				</Card>
			</TabsContent>

			<TabsContent value="counters">
				<div className="grid grid-cols-1 gap-3 lg:grid-cols-2">
					<Card className="rounded-xl">
						<CardHeader>
							<CardTitle className="text-sm">Requests by method</CardTitle>
						</CardHeader>
						<CardContent>
							<CountersTab client={client} />
						</CardContent>
					</Card>
					<Card className="rounded-xl">
						<CardHeader>
							<CardTitle className="text-sm">Method distribution</CardTitle>
						</CardHeader>
						<CardContent className="h-64">
							<MethodDonut client={client} />
						</CardContent>
					</Card>
				</div>
			</TabsContent>

			<TabsContent value="errors">
				<Card className="rounded-xl">
					<CardContent className="pt-4">
						<ErrorsTab
							events={events}
							totalErrors={totalErrors}
							isActive={isActive}
						/>
					</CardContent>
				</Card>
			</TabsContent>

			<TabsContent value="watches">
				<Card className="rounded-xl">
					<CardContent className="pt-6">
						<WatchesTab client={client} isActive={isActive} />
					</CardContent>
				</Card>
			</TabsContent>

			<TabsContent value="sessions">
				<Card className="rounded-xl">
					<CardContent className="pt-4">
						<SessionsTab client={client} />
					</CardContent>
				</Card>
			</TabsContent>
		</Tabs>
	);
}

/** Shared table for activity and error event lists. */
function EventTable({
	events,
	highlightErrors = false,
}: {
	events: ClientEvent[];
	highlightErrors?: boolean;
}) {
	return (
		<div className="overflow-hidden rounded-md border">
			<Table>
				<TableHeader>
					<TableRow>
						<TableHead className="w-32">Time</TableHead>
						<TableHead>Method</TableHead>
						<TableHead>Key</TableHead>
						<TableHead className="w-24 text-right">Duration</TableHead>
						<TableHead>Error</TableHead>
					</TableRow>
				</TableHeader>
				<TableBody>
					{events.map((ev, idx) => {
						const t = ev.timestamp ? new Date(tsToMs(ev.timestamp)) : null;
						const rowClass = highlightErrors
							? "bg-destructive/5"
							: ev.error
								? "bg-destructive/5"
								: undefined;
						return (
							<TableRow
								// biome-ignore lint/suspicious/noArrayIndexKey: events have no stable id
								key={`${idx}-${tsToMs(ev.timestamp)}-${ev.method}`}
								className={rowClass}
							>
								<TableCell className="whitespace-nowrap text-muted-foreground text-xs">
									{t ? t.toLocaleTimeString() : "—"}
								</TableCell>
								<TableCell className="font-mono text-xs">{ev.method}</TableCell>
								<TableCell className="font-mono text-muted-foreground text-xs">
									{ev.key || "—"}
								</TableCell>
								<TableCell className="whitespace-nowrap text-right text-xs">
									{formatRpcDuration(ev.duration)}
								</TableCell>
								<TableCell
									className={
										highlightErrors
											? "font-medium text-destructive text-xs"
											: "text-destructive text-xs"
									}
								>
									{ev.error}
								</TableCell>
							</TableRow>
						);
					})}
				</TableBody>
			</Table>
		</div>
	);
}

function ActivityTab({
	events,
	isActive,
}: {
	events: ClientEvent[];
	isActive: boolean;
}) {
	if (events.length === 0) {
		return (
			<Empty>
				<EmptyHeader>
					<EmptyMedia variant="icon">
						<Eye />
					</EmptyMedia>
					<EmptyTitle>No activity yet</EmptyTitle>
					<EmptyDescription>
						{isActive
							? "Waiting for the next RPC..."
							: "This client did not produce any recorded activity."}
					</EmptyDescription>
				</EmptyHeader>
			</Empty>
		);
	}

	return <EventTable events={events} />;
}

function ErrorsTab({
	events,
	totalErrors,
	isActive,
}: {
	events: ClientEvent[];
	totalErrors: number;
	isActive: boolean;
}) {
	const errorEvents = events.filter((ev) => ev.error !== "");

	// Empty cases — pick a message that matches the actual state.
	if (errorEvents.length === 0) {
		if (totalErrors === 0) {
			return (
				<Empty>
					<EmptyHeader>
						<EmptyMedia variant="icon">
							<CheckCircle2 />
						</EmptyMedia>
						<EmptyTitle>No errors</EmptyTitle>
						<EmptyDescription>
							This client has not produced any failed RPCs.
						</EmptyDescription>
					</EmptyHeader>
				</Empty>
			);
		}

		if (!isActive) {
			// Historical clients: per-event detail isn't persisted, only the count is.
			return (
				<Empty>
					<EmptyHeader>
						<EmptyMedia variant="icon">
							<AlertTriangle />
						</EmptyMedia>
						<EmptyTitle>
							{totalErrors} error{totalErrors === 1 ? "" : "s"} during this
							session
						</EmptyTitle>
						<EmptyDescription>
							Per-error detail is not retained after a client disconnects — only
							the count is preserved in history.
						</EmptyDescription>
					</EmptyHeader>
				</Empty>
			);
		}

		// Active client — errors happened but rolled out of the in-memory buffer.
		return (
			<Empty>
				<EmptyHeader>
					<EmptyMedia variant="icon">
						<AlertTriangle />
					</EmptyMedia>
					<EmptyTitle>No recent errors</EmptyTitle>
					<EmptyDescription>
						{totalErrors} earlier error{totalErrors === 1 ? "" : "s"} occurred
						but they have rolled out of the in-memory event buffer.
					</EmptyDescription>
				</EmptyHeader>
			</Empty>
		);
	}

	const truncated = errorEvents.length < totalErrors;

	return (
		<div className="space-y-3">
			{truncated && (
				<p className="text-muted-foreground text-xs">
					Showing {errorEvents.length} of {totalErrors} total errors.{" "}
					{isActive
						? "Older errors have rolled out of the in-memory buffer."
						: "Per-error detail is not retained after disconnect; older entries are gone."}
				</p>
			)}
			<EventTable events={errorEvents} highlightErrors />
		</div>
	);
}

interface CounterRow {
	method: string;
	count: number;
}

const counterColumns: ColumnDef<CounterRow>[] = [
	{
		accessorKey: "method",
		header: ({ column }) => (
			<SortableHeader column={column}>Method</SortableHeader>
		),
		cell: ({ row }) => (
			<span className="font-mono text-xs">{row.original.method}</span>
		),
	},
	{
		accessorKey: "count",
		header: ({ column }) => (
			<SortableHeader column={column}>Count</SortableHeader>
		),
		cell: ({ row }) => (
			<span className="tabular-nums">
				{row.original.count.toLocaleString()}
			</span>
		),
	},
];

function CountersTab({ client }: { client: Client }) {
	const [sorting, setSorting] = useState<SortingState>([
		{ id: "method", desc: false },
	]);

	const data = useMemo<CounterRow[]>(() => {
		return Object.entries(client.requestCounts ?? {}).map(
			([method, count]) => ({
				method,
				count: Number(count),
			}),
		);
	}, [client.requestCounts]);

	const table = useReactTable({
		data,
		columns: counterColumns,
		state: { sorting },
		onSortingChange: (updater) => {
			const next = typeof updater === "function" ? updater(sorting) : updater;
			setSorting(next);
		},
		getCoreRowModel: getCoreRowModel(),
		getSortedRowModel: getSortedRowModel(),
	});

	if (data.length === 0) {
		return (
			<Empty>
				<EmptyHeader>
					<EmptyMedia variant="icon">
						<BarChart3 />
					</EmptyMedia>
					<EmptyTitle>No requests recorded yet</EmptyTitle>
				</EmptyHeader>
			</Empty>
		);
	}

	return (
		<Table>
			<TableHeader>
				{table.getHeaderGroups().map((group) => (
					<TableRow key={group.id}>
						{group.headers.map((header) => (
							<TableHead key={header.id}>
								{flexRender(
									header.column.columnDef.header,
									header.getContext(),
								)}
							</TableHead>
						))}
					</TableRow>
				))}
			</TableHeader>
			<TableBody>
				{table.getRowModel().rows.map((row) => (
					<TableRow key={row.id}>
						{row.getVisibleCells().map((cell) => (
							<TableCell key={cell.id}>
								{flexRender(cell.column.columnDef.cell, cell.getContext())}
							</TableCell>
						))}
					</TableRow>
				))}
			</TableBody>
		</Table>
	);
}

function MethodDonut({ client }: { client: Client }) {
	const data = useMemo(() => {
		const entries = Object.entries(client.requestCounts ?? {})
			.map(([method, count]) => ({
				name: shortMethod(method),
				value: Number(count),
			}))
			.sort((a, b) => b.value - a.value);

		if (entries.length <= 5) return entries;

		const top = entries.slice(0, 5);
		const otherSum = entries.slice(5).reduce((sum, e) => sum + e.value, 0);
		return [...top, { name: "other", value: otherSum }];
	}, [client.requestCounts]);

	if (data.length === 0 || data.every((d) => d.value === 0)) {
		return (
			<Empty>
				<EmptyHeader>
					<EmptyMedia variant="icon">
						<BarChart3 />
					</EmptyMedia>
					<EmptyTitle>No data yet</EmptyTitle>
				</EmptyHeader>
			</Empty>
		);
	}

	return (
		<ResponsiveContainer width="100%" height="100%">
			<PieChart>
				<RechartsTooltip
					contentStyle={{
						borderRadius: 8,
						background: "hsl(var(--popover))",
						border: "1px solid hsl(var(--border))",
						fontSize: 12,
					}}
				/>
				<Pie
					data={data}
					dataKey="value"
					nameKey="name"
					innerRadius="55%"
					outerRadius="85%"
					paddingAngle={2}
					isAnimationActive={false}
					label={(entry) => entry.name}
					labelLine={false}
				>
					{data.map((_, i) => (
						<Cell
							// biome-ignore lint/suspicious/noArrayIndexKey: stable position-based color
							key={i}
							fill={DONUT_COLORS[i % DONUT_COLORS.length]}
						/>
					))}
				</Pie>
			</PieChart>
		</ResponsiveContainer>
	);
}

function shortMethod(fullMethod: string): string {
	// "/etcdserverpb.KV/Put" → "KV/Put"
	const parts = fullMethod.split("/").filter(Boolean);
	if (parts.length < 2) return fullMethod;
	const svc = parts[0].split(".").pop() ?? parts[0];
	return `${svc}/${parts[1]}`;
}

function WatchesTab({
	client,
	isActive,
}: {
	client: Client;
	isActive: boolean;
}) {
	const watches = client.activeWatchList ?? [];

	if (!isActive) {
		return (
			<Empty>
				<EmptyHeader>
					<EmptyMedia variant="icon">
						<Eye />
					</EmptyMedia>
					<EmptyTitle>Watches not retained</EmptyTitle>
					<EmptyDescription>
						Per-watch detail is in-memory only and is dropped when a client
						disconnects. The final count was{" "}
						<span className="font-medium">{client.activeWatches}</span>.
					</EmptyDescription>
				</EmptyHeader>
			</Empty>
		);
	}

	if (watches.length === 0) {
		return (
			<Empty>
				<EmptyHeader>
					<EmptyMedia variant="icon">
						<Eye />
					</EmptyMedia>
					<EmptyTitle>No active watches</EmptyTitle>
					<EmptyDescription>
						This client has no open Watch streams.
					</EmptyDescription>
				</EmptyHeader>
			</Empty>
		);
	}

	// Sort by created_at desc — newest first.
	const sorted = [...watches].sort(
		(a, b) => tsToMs(b.createdAt) - tsToMs(a.createdAt),
	);

	return (
		<div className="overflow-hidden rounded-md border">
			<Table>
				<TableHeader>
					<TableRow>
						<TableHead className="w-16">ID</TableHead>
						<TableHead className="w-24">Type</TableHead>
						<TableHead className="w-40">Namespace</TableHead>
						<TableHead>Path</TableHead>
						<TableHead className="w-28">Start rev</TableHead>
						<TableHead className="w-28">Created</TableHead>
						<TableHead className="w-28">Flags</TableHead>
					</TableRow>
				</TableHeader>
				<TableBody>
					{sorted.map((w) => {
						const target = classifyWatch(w.startKey, w.endKey);
						return (
							<TableRow key={String(w.watchId)}>
								<TableCell className="font-mono text-muted-foreground text-xs">
									{String(w.watchId)}
								</TableCell>
								<TableCell>
									<WatchTypeBadge target={target} />
								</TableCell>
								<TableCell>
									<WatchNamespaceCell target={target} />
								</TableCell>
								<TableCell>
									<WatchPathCell target={target} />
								</TableCell>
								<TableCell className="font-mono text-xs tabular-nums">
									{w.startRevision > 0n ? String(w.startRevision) : "now"}
								</TableCell>
								<TableCell className="text-muted-foreground text-xs">
									{w.createdAt ? timeAgo(new Date(tsToMs(w.createdAt))) : "—"}
								</TableCell>
								<TableCell>
									<div className="flex flex-wrap gap-1">
										{w.prevKv && (
											<Badge variant="outline" className="text-[10px]">
												prev_kv
											</Badge>
										)}
										{w.progressNotify && (
											<Badge variant="outline" className="text-[10px]">
												progress
											</Badge>
										)}
									</div>
								</TableCell>
							</TableRow>
						);
					})}
				</TableBody>
			</Table>
		</div>
	);
}

function WatchTypeBadge({ target }: { target: WatchTarget }) {
	switch (target.kind) {
		case "key":
			return (
				<Badge variant="secondary" className="text-[10px]">
					key
				</Badge>
			);
		case "prefix":
			return (
				<Badge variant="secondary" className="text-[10px]">
					prefix
				</Badge>
			);
		case "all-in-namespace":
			return (
				<Badge variant="secondary" className="text-[10px]">
					all in ns
				</Badge>
			);
		case "range":
			return (
				<Badge variant="secondary" className="text-[10px]">
					range
				</Badge>
			);
		case "scan-all":
			return (
				<Badge variant="secondary" className="text-[10px]">
					≥ key
				</Badge>
			);
		case "raw":
			return (
				<Badge variant="outline" className="text-[10px]">
					raw
				</Badge>
			);
	}
}

function WatchNamespaceCell({ target }: { target: WatchTarget }) {
	if (target.kind === "raw") {
		return <span className="text-muted-foreground italic">—</span>;
	}
	return (
		<Badge variant="outline" className="font-mono text-xs">
			{target.namespace}
		</Badge>
	);
}

function WatchPathCell({ target }: { target: WatchTarget }) {
	switch (target.kind) {
		case "key":
			// Single config file — the most common case for "I want this exact key"
			return <span className="font-mono text-xs">{target.path}</span>;
		case "prefix":
			return (
				<span className="font-mono text-xs">
					{target.path}
					<span className="text-muted-foreground">*</span>
				</span>
			);
		case "all-in-namespace":
			return <span className="text-muted-foreground italic">all configs</span>;
		case "range":
			return (
				<span className="font-mono text-xs">
					{target.startPath} <span className="text-muted-foreground">→</span>{" "}
					{target.endPath}
				</span>
			);
		case "scan-all":
			return (
				<span className="font-mono text-xs">
					{target.path}{" "}
					<span className="text-muted-foreground">and beyond</span>
				</span>
			);
		case "raw":
			return (
				<span className="font-mono text-muted-foreground text-xs">
					{target.startKey}
					{target.endKey ? ` → ${target.endKey}` : ""}
				</span>
			);
	}
}

function SessionsTab({ client }: { client: Client }) {
	const navigate = useNavigate();
	const enabled = !!client.clientName;

	const q = useQuery(
		listClientSessions,
		{
			clientName: client.clientName,
			k8sNamespace: client.k8sNamespace,
			currentId: client.id,
			limit: 50,
		},
		{ enabled },
	);

	if (!enabled) {
		return (
			<Empty>
				<EmptyHeader>
					<EmptyMedia variant="icon">
						<HistoryIcon />
					</EmptyMedia>
					<EmptyTitle>Anonymous client</EmptyTitle>
					<EmptyDescription>
						Sessions can only be correlated for clients that send the{" "}
						<code className="rounded bg-muted px-1 py-0.5 text-xs">
							x-client-name
						</code>{" "}
						metadata header.
					</EmptyDescription>
				</EmptyHeader>
			</Empty>
		);
	}

	if (q.isLoading) {
		return (
			<div className="space-y-2">
				{Array.from({ length: 3 }).map((_, i) => (
					// biome-ignore lint/suspicious/noArrayIndexKey: skeletons
					<Skeleton key={i} className="h-10 w-full rounded-lg" />
				))}
			</div>
		);
	}

	const sessions = q.data?.sessions ?? [];
	if (sessions.length === 0) {
		return (
			<Empty>
				<EmptyHeader>
					<EmptyMedia variant="icon">
						<HistoryIcon />
					</EmptyMedia>
					<EmptyTitle>No previous sessions</EmptyTitle>
					<EmptyDescription>
						This is the first connection from{" "}
						<span className="font-mono">{client.clientName}</span>
						{client.k8sNamespace ? (
							<>
								{" "}
								in namespace{" "}
								<span className="font-mono">{client.k8sNamespace}</span>
							</>
						) : null}
						.
					</EmptyDescription>
				</EmptyHeader>
			</Empty>
		);
	}

	return (
		<Table>
			<TableHeader>
				<TableRow>
					<TableHead>Pod</TableHead>
					<TableHead>Disconnected</TableHead>
					<TableHead className="text-right">Duration</TableHead>
					<TableHead className="text-right">Requests</TableHead>
					<TableHead className="text-right">Errors</TableHead>
				</TableRow>
			</TableHeader>
			<TableBody>
				{sessions.map((s) => {
					const start = tsToMs(s.connectedAt);
					const end = tsToMs(s.disconnectedAt);
					const duration = start && end ? formatUptime(start, end) : "—";
					return (
						<TableRow
							key={s.id}
							className="cursor-pointer hover:bg-muted/50"
							onClick={() => navigate(`/clients/${s.id}`)}
						>
							<TableCell className="font-mono text-xs">
								{s.k8sPod || "—"}
							</TableCell>
							<TableCell className="text-xs">
								{end ? timeAgo(new Date(end)) : "—"}
							</TableCell>
							<TableCell className="text-right text-xs">{duration}</TableCell>
							<TableCell className="text-right text-xs">
								{totalRequests(s).toLocaleString()}
							</TableCell>
							<TableCell className="text-right text-xs">
								<span
									className={
										Number(s.errorCount) > 0
											? "font-medium text-destructive"
											: "text-muted-foreground"
									}
								>
									{Number(s.errorCount)}
								</span>
							</TableCell>
						</TableRow>
					);
				})}
			</TableBody>
		</Table>
	);
}

import {
	type ColumnDef,
	flexRender,
	getCoreRowModel,
	getSortedRowModel,
	type SortingState,
	useReactTable,
} from "@tanstack/react-table";
import { useNavigate } from "react-router";
import { SkeletonList } from "@/components/skeleton-list";
import { SortableHeader } from "@/components/sortable-header";
import { Badge } from "@/components/ui/badge";
import {
	Table,
	TableBody,
	TableCell,
	TableHead,
	TableHeader,
	TableRow,
} from "@/components/ui/table";
import type { Client } from "@/gen/elara/clients/v1/clients_pb";
import { totalRequests } from "@/lib/client";
import { isLive } from "@/lib/duration";
import { timeAgo, tsToMs } from "@/lib/time";

interface ClientsTableProps {
	clients: Client[];
	isLoading: boolean;
	mode: "active" | "history";
	sorting: SortingState;
	onSortingChange: (s: SortingState) => void;
	emptySlot?: React.ReactNode;
}

function nameCell(c: Client): React.ReactNode {
	if (c.clientName) {
		return (
			<div className="flex flex-col">
				<span className="font-medium">{c.clientName}</span>
				{c.clientVersion && (
					<span className="text-muted-foreground text-xs">
						v{c.clientVersion}
					</span>
				)}
			</div>
		);
	}
	return (
		<div className="flex flex-col">
			<span className="text-muted-foreground italic">unknown</span>
			<span className="text-muted-foreground text-xs">
				{c.userAgent || "no user-agent"}
			</span>
		</div>
	);
}

function podCell(c: Client): React.ReactNode {
	if (c.k8sPod || c.k8sNamespace) {
		return (
			<div className="flex flex-col">
				{c.k8sPod && <span className="font-mono text-xs">{c.k8sPod}</span>}
				{c.k8sNamespace && (
					<Badge variant="outline" className="w-fit text-[10px]">
						{c.k8sNamespace}
					</Badge>
				)}
			</div>
		);
	}
	return (
		<span className="font-mono text-muted-foreground text-xs">
			{c.peerAddress}
		</span>
	);
}

function errorsCell(c: Client): React.ReactNode {
	const n = Number(c.errorCount);
	return n > 0 ? (
		<span className="font-medium text-destructive">{n}</span>
	) : (
		<span className="text-muted-foreground">0</span>
	);
}

const clientColumn: ColumnDef<Client> = {
	accessorKey: "clientName",
	header: ({ column }) => (
		<SortableHeader column={column}>Client</SortableHeader>
	),
	cell: ({ row }) => nameCell(row.original),
	sortingFn: (a, b) =>
		(a.original.clientName || "~").localeCompare(b.original.clientName || "~"),
};

const podColumn: ColumnDef<Client> = {
	id: "pod",
	header: "Pod / Peer",
	cell: ({ row }) => podCell(row.original),
	enableSorting: false,
};

const requestsColumn: ColumnDef<Client> = {
	id: "requests",
	accessorFn: (c) => totalRequests(c),
	header: ({ column }) => (
		<SortableHeader column={column}>Requests</SortableHeader>
	),
	cell: ({ row }) => totalRequests(row.original).toLocaleString(),
};

const errorsColumn: ColumnDef<Client> = {
	accessorKey: "errorCount",
	header: ({ column }) => (
		<SortableHeader column={column}>Errors</SortableHeader>
	),
	cell: ({ row }) => errorsCell(row.original),
};

const statusColumn: ColumnDef<Client> = {
	id: "status",
	header: "",
	cell: ({ row }) => {
		const live = isLive(tsToMs(row.original.lastActivityAt));
		return (
			<span
				className={
					live
						? "inline-block h-2 w-2 animate-pulse rounded-full bg-emerald-500"
						: "inline-block h-2 w-2 rounded-full bg-muted-foreground/40"
				}
			/>
		);
	},
	enableSorting: false,
};

const connectedAtColumn: ColumnDef<Client> = {
	accessorKey: "connectedAt",
	header: ({ column }) => (
		<SortableHeader column={column}>Connected</SortableHeader>
	),
	cell: ({ row }) => {
		const ms = tsToMs(row.original.connectedAt);
		return ms ? timeAgo(new Date(ms)) : "—";
	},
	sortingFn: (a, b) =>
		tsToMs(a.original.connectedAt) - tsToMs(b.original.connectedAt),
};

const lastActivityColumn: ColumnDef<Client> = {
	accessorKey: "lastActivityAt",
	header: ({ column }) => (
		<SortableHeader column={column}>Last activity</SortableHeader>
	),
	cell: ({ row }) => {
		const ms = tsToMs(row.original.lastActivityAt);
		return ms ? timeAgo(new Date(ms)) : "—";
	},
	sortingFn: (a, b) =>
		tsToMs(a.original.lastActivityAt) - tsToMs(b.original.lastActivityAt),
};

const watchesColumn: ColumnDef<Client> = {
	accessorKey: "activeWatches",
	header: ({ column }) => (
		<SortableHeader column={column}>Watches</SortableHeader>
	),
	cell: ({ row }) => row.original.activeWatches,
};

const disconnectedAtColumn: ColumnDef<Client> = {
	accessorKey: "disconnectedAt",
	header: ({ column }) => (
		<SortableHeader column={column}>Disconnected</SortableHeader>
	),
	cell: ({ row }) => {
		const ms = tsToMs(row.original.disconnectedAt);
		return ms ? timeAgo(new Date(ms)) : "—";
	},
	sortingFn: (a, b) =>
		tsToMs(a.original.disconnectedAt) - tsToMs(b.original.disconnectedAt),
};

const durationColumn: ColumnDef<Client> = {
	id: "duration",
	header: "Duration",
	cell: ({ row }) => {
		const start = tsToMs(row.original.connectedAt);
		const end = tsToMs(row.original.disconnectedAt);
		if (!start || !end) return "—";
		const seconds = Math.max(0, Math.floor((end - start) / 1000));
		if (seconds < 60) return `${seconds}s`;
		if (seconds < 3600) return `${Math.floor(seconds / 60)}m`;
		if (seconds < 86_400) return `${Math.floor(seconds / 3600)}h`;
		return `${Math.floor(seconds / 86_400)}d`;
	},
	enableSorting: false,
};

const activeColumns: ColumnDef<Client>[] = [
	statusColumn,
	clientColumn,
	podColumn,
	connectedAtColumn,
	lastActivityColumn,
	watchesColumn,
	requestsColumn,
	errorsColumn,
];

const historyColumns: ColumnDef<Client>[] = [
	clientColumn,
	podColumn,
	disconnectedAtColumn,
	durationColumn,
	requestsColumn,
	errorsColumn,
];

export function ClientsTable({
	clients,
	isLoading,
	mode,
	sorting,
	onSortingChange,
	emptySlot,
}: ClientsTableProps) {
	const navigate = useNavigate();

	const table = useReactTable({
		data: clients,
		columns: mode === "active" ? activeColumns : historyColumns,
		state: { sorting },
		onSortingChange: (updater) => {
			const next = typeof updater === "function" ? updater(sorting) : updater;
			onSortingChange(next);
		},
		getCoreRowModel: getCoreRowModel(),
		getSortedRowModel: getSortedRowModel(),
	});

	if (isLoading) {
		return <SkeletonList count={5} className="h-12 w-full rounded-lg" />;
	}

	if (clients.length === 0 && emptySlot) {
		return <>{emptySlot}</>;
	}

	return (
		<Table>
			<TableHeader>
				{table.getHeaderGroups().map((headerGroup) => (
					<TableRow key={headerGroup.id}>
						{headerGroup.headers.map((header) => (
							<TableHead key={header.id}>
								{header.isPlaceholder
									? null
									: flexRender(
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
					<TableRow
						key={row.id}
						className="cursor-pointer hover:bg-muted/50"
						tabIndex={0}
						role="link"
						onClick={(e) => {
							// Cmd/Ctrl+click → new tab via Link semantics
							const url = `/clients/${row.original.id}`;
							if (e.metaKey || e.ctrlKey) {
								window.open(url, "_blank");
								return;
							}
							navigate(url);
						}}
						onKeyDown={(e) => {
							if (e.key === "Enter" || e.key === " ") {
								e.preventDefault();
								navigate(`/clients/${row.original.id}`);
							}
						}}
					>
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

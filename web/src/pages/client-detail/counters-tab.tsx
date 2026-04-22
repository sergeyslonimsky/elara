import {
	type ColumnDef,
	flexRender,
	getCoreRowModel,
	getSortedRowModel,
	type SortingState,
	useReactTable,
} from "@tanstack/react-table";
import { BarChart3 } from "lucide-react";
import { useMemo, useState } from "react";
import { SortableHeader } from "@/components/sortable-header";
import {
	Empty,
	EmptyHeader,
	EmptyMedia,
	EmptyTitle,
} from "@/components/ui/empty";
import {
	Table,
	TableBody,
	TableCell,
	TableHead,
	TableHeader,
	TableRow,
} from "@/components/ui/table";
import type { Client } from "@/gen/elara/clients/v1/clients_pb";

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

export function CountersTab({ client }: { client: Client }) {
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

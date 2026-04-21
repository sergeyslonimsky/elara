import { timestampDate } from "@bufbuild/protobuf/wkt";
import type { ColumnDef, SortingState } from "@tanstack/react-table";
import { ChevronRight, FilePlus, FileText, Folder, Lock } from "lucide-react";
import { useNavigate } from "react-router";
import { DataTable } from "@/components/data-table";
import { SortableHeader } from "@/components/sortable-header";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { SortDirection } from "@/gen/elara/common/v1/common_pb";
import { type DirectoryEntry, Format } from "@/gen/elara/config/v1/config_pb";
import { formatLabel } from "@/lib/format";
import { timeAgo, tsToMs } from "@/lib/time";

interface DirectoryTableProps {
	namespace: string;
	currentPath: string;
	entries: DirectoryEntry[];
	isLoading: boolean;
	sorting: SortingState;
	onSortingChange: (sorting: SortingState) => void;
}

const columns: ColumnDef<DirectoryEntry>[] = [
	{
		accessorKey: "name",
		header: ({ column }) => (
			<SortableHeader column={column}>Name</SortableHeader>
		),
		cell: ({ row }) => {
			const entry = row.original;
			return (
				<div className="flex items-center gap-2 font-medium">
					{entry.isFile ? (
						<FileText className="h-4 w-4 shrink-0 text-muted-foreground" />
					) : (
						<Folder className="h-4 w-4 shrink-0 text-blue-500" />
					)}
					{entry.name}
					{entry.isFile && entry.locked && (
						<Lock className="h-3 w-3 shrink-0 text-amber-500" />
					)}
					{!entry.isFile && (
						<ChevronRight className="ml-auto h-4 w-4 text-muted-foreground" />
					)}
				</div>
			);
		},
	},
	{
		id: "type",
		header: "Type",
		cell: ({ row }) => {
			const entry = row.original;
			if (entry.isFile) {
				return entry.format !== Format.UNSPECIFIED ? (
					<Badge variant="secondary">{formatLabel(entry.format)}</Badge>
				) : null;
			}
			return <span className="text-muted-foreground text-sm">Folder</span>;
		},
		enableSorting: false,
	},
	{
		id: "info",
		header: "Info",
		cell: ({ row }) => {
			const entry = row.original;
			if (entry.isFile) {
				return (
					<span className="text-muted-foreground text-sm">
						v{entry.version}
					</span>
				);
			}
			return (
				<span className="text-muted-foreground text-sm">
					{entry.childCount} item{entry.childCount !== 1 ? "s" : ""}
				</span>
			);
		},
		enableSorting: false,
	},
	{
		id: "modified",
		accessorFn: (row) => tsToMs(row.updatedAt),
		header: ({ column }) => (
			<div className="text-right">
				<SortableHeader column={column}>Modified</SortableHeader>
			</div>
		),
		cell: ({ row }) => {
			const entry = row.original;
			return (
				<div className="text-right text-muted-foreground text-sm">
					{entry.updatedAt && timeAgo(timestampDate(entry.updatedAt))}
				</div>
			);
		},
	},
];

export function DirectoryTable({
	namespace,
	currentPath,
	entries,
	isLoading,
	sorting,
	onSortingChange,
}: DirectoryTableProps) {
	const navigate = useNavigate();

	if (isLoading) {
		return (
			<div className="space-y-2 p-4 border rounded-xl bg-card">
				{Array.from({ length: 5 }).map((_, i) => (
					// biome-ignore lint/suspicious/noArrayIndexKey: skeleton placeholder
					<Skeleton key={i} className="h-10 w-full" />
				))}
			</div>
		);
	}

	if (entries.length === 0) {
		const newPath =
			currentPath === "/"
				? `/config/new/${namespace}`
				: `/config/new/${namespace}${currentPath}`;

		return (
			<div className="flex flex-col items-center justify-center gap-3 py-16 text-muted-foreground border rounded-xl bg-card">
				<Folder className="h-12 w-12" />
				<p className="text-lg font-medium">Empty directory</p>
				<p className="text-sm">No configs found at this path</p>
				<Button variant="outline" size="sm" onClick={() => navigate(newPath)}>
					<FilePlus className="mr-1 h-4 w-4" />
					New Config
				</Button>
			</div>
		);
	}

	return (
		<DataTable
			columns={columns}
			data={entries}
			sorting={sorting}
			onSortingChange={onSortingChange}
			onRowClick={(entry) => {
				const href = entry.isFile
					? `/config/${namespace}${entry.fullPath}`
					: `/browse/${namespace}${entry.fullPath}`;
				navigate(href);
			}}
		/>
	);
}

// Convert TanStack sorting state to server sort params.
export function sortingToParams(sorting: SortingState): {
	field: string;
	direction: SortDirection;
} {
	if (sorting.length === 0) {
		return { field: "", direction: SortDirection.UNSPECIFIED };
	}

	const s = sorting[0];
	const field = s.id === "modified" ? "modified" : "name";
	const direction = s.desc ? SortDirection.DESC : SortDirection.ASC;

	return { field, direction };
}

import { timestampDate } from "@bufbuild/protobuf/wkt";
import type { ColumnDef, SortingState } from "@tanstack/react-table";
import { ChevronRight, FilePlus, FileText, Folder, Lock } from "lucide-react";
import { useNavigate } from "react-router";
import { DataTable } from "@/components/data-table";
import { LockAwareButton } from "@/components/lock-aware-button";
import { SkeletonList } from "@/components/skeleton-list";
import { SortableHeader } from "@/components/sortable-header";
import { Badge } from "@/components/ui/badge";
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
	namespaceLocked?: boolean;
}

const columns: ColumnDef<DirectoryEntry>[] = [
	{
		accessorKey: "name",
		header: ({ column }) => (
			<SortableHeader column={column}>Name</SortableHeader>
		),
		cell: ({ row }) => {
			const entry = row.original;
			const showLock = entry.isFile && (entry.locked || entry.namespaceLocked);
			const lockTitle = entry.namespaceLocked
				? "Namespace is locked"
				: "Config is locked";
			return (
				<div className="flex items-center gap-2 font-medium">
					{entry.isFile ? (
						<FileText className="h-4 w-4 shrink-0 text-muted-foreground" />
					) : (
						<Folder className="h-4 w-4 shrink-0 text-blue-500" />
					)}
					{entry.name}
					{showLock && (
						<Lock
							className="h-3 w-3 shrink-0 text-amber-500"
							aria-label={lockTitle}
						/>
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
					{entry.childCount} item{entry.childCount === 1 ? "" : "s"}
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
	namespaceLocked = false,
}: Readonly<DirectoryTableProps>) {
	const navigate = useNavigate();

	if (isLoading) {
		return (
			<SkeletonList
				count={5}
				wrapperClassName="space-y-2 p-4 border rounded-xl bg-card"
			/>
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
				<LockAwareButton
					variant="outline"
					size="sm"
					locked={namespaceLocked}
					lockedReason={`Namespace "${namespace}" is locked`}
					onClick={() => navigate(newPath)}
				>
					<FilePlus className="mr-1 h-4 w-4" />
					New Config
				</LockAwareButton>
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

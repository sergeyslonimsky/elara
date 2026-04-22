import { timestampDate } from "@bufbuild/protobuf/wkt";
import type { ColumnDef } from "@tanstack/react-table";
import { Folder } from "lucide-react";
import { SortableHeader } from "@/components/sortable-header";
import { Badge } from "@/components/ui/badge";
import type { Namespace } from "@/gen/elara/namespace/v1/namespace_pb";
import { timeAgo, tsToMs } from "@/lib/time";

export const nsColumns: ColumnDef<Namespace>[] = [
	{
		accessorKey: "name",
		header: ({ column }) => (
			<SortableHeader column={column}>Name</SortableHeader>
		),
		cell: ({ row }) => (
			<div className="flex items-center gap-2 font-medium">
				<Folder className="h-4 w-4 shrink-0 text-blue-500" />
				{row.original.name}
			</div>
		),
	},
	{
		id: "type",
		header: "Type",
		cell: () => <Badge variant="outline">Namespace</Badge>,
		enableSorting: false,
	},
	{
		id: "info",
		header: "Info",
		cell: ({ row }) => (
			<span className="text-muted-foreground text-sm">
				{row.original.configCount} config
				{row.original.configCount !== 1 ? "s" : ""}
			</span>
		),
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
		cell: ({ row }) => (
			<div className="text-right text-muted-foreground text-sm">
				{row.original.updatedAt &&
					timeAgo(timestampDate(row.original.updatedAt))}
			</div>
		),
	},
];

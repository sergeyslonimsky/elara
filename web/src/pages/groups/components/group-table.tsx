import { timestampDate } from "@bufbuild/protobuf/wkt";
import type { ColumnDef } from "@tanstack/react-table";
import { format } from "date-fns";
import { Settings2, Trash2, UsersRound } from "lucide-react";
import { DataTable } from "@/components/data-table";
import { Button } from "@/components/ui/button";
import {
	Empty,
	EmptyDescription,
	EmptyHeader,
	EmptyMedia,
	EmptyTitle,
} from "@/components/ui/empty";
import type { Group } from "@/gen/elara/group/v1/group_pb";

interface GroupTableProps {
	groups: Group[];
	isLoading: boolean;
	query?: string;
	onEdit: (group: Group) => void;
	onDelete: (group: Group) => void;
}

export function GroupTable({
	groups,
	isLoading,
	query,
	onEdit,
	onDelete,
}: GroupTableProps) {
	const columns: ColumnDef<Group>[] = [
		{
			accessorKey: "name",
			header: "Name",
			cell: ({ row }) => (
				<div className="flex items-center gap-2">
					<UsersRound className="h-4 w-4 text-muted-foreground" />
					<span className="font-medium">{row.original.name}</span>
				</div>
			),
		},
		{
			accessorKey: "id",
			header: "ID",
			cell: ({ row }) => (
				<span className="font-mono text-xs text-muted-foreground">
					{row.original.id}
				</span>
			),
		},
		{
			id: "members",
			header: "Members",
			cell: ({ row }) => <span>{row.original.members.length} member(s)</span>,
		},
		{
			accessorKey: "createdAt",
			header: "Created At",
			cell: ({ row }) => {
				const date = row.original.createdAt
					? timestampDate(row.original.createdAt)
					: null;
				return date ? format(date, "PPP p") : "-";
			},
		},
		{
			id: "actions",
			header: () => <div className="text-right">Actions</div>,
			cell: ({ row }) => (
				<div className="flex justify-end gap-1">
					<Button
						variant="ghost"
						size="icon-sm"
						onClick={(e) => {
							e.stopPropagation();
							onEdit(row.original);
						}}
						title="Edit Group"
					>
						<Settings2 className="h-4 w-4" />
					</Button>
					<Button
						variant="ghost"
						size="icon-sm"
						className="text-destructive hover:text-destructive"
						onClick={(e) => {
							e.stopPropagation();
							onDelete(row.original);
						}}
						title="Delete Group"
					>
						<Trash2 className="h-4 w-4" />
					</Button>
				</div>
			),
		},
	];

	if (!isLoading && groups.length === 0) {
		return (
			<Empty className="py-12">
				<EmptyHeader>
					<EmptyMedia variant="icon">
						<UsersRound />
					</EmptyMedia>
					<EmptyTitle>{query ? "No groups found" : "No groups"}</EmptyTitle>
					<EmptyDescription>
						{query
							? `No results for "${query}"`
							: "There are no groups in the system yet."}
					</EmptyDescription>
				</EmptyHeader>
			</Empty>
		);
	}

	return (
		<DataTable
			columns={columns}
			data={groups}
			nameColumnWidth="w-[30%]"
			onRowClick={(group) => onEdit(group)}
		/>
	);
}

import { timestampDate } from "@bufbuild/protobuf/wkt";
import type { ColumnDef } from "@tanstack/react-table";
import { format } from "date-fns";
import { UserCog, User as UserIcon } from "lucide-react";
import { DataTable } from "@/components/data-table";
import { Button } from "@/components/ui/button";
import {
	Empty,
	EmptyDescription,
	EmptyHeader,
	EmptyMedia,
	EmptyTitle,
} from "@/components/ui/empty";
import type { User } from "@/gen/elara/user/v1/user_pb";

interface UserTableProps {
	users: User[];
	isLoading: boolean;
	query?: string;
	onEditGroups: (user: User) => void;
}

export function UserTable({
	users,
	isLoading,
	query,
	onEditGroups,
}: UserTableProps) {
	const columns: ColumnDef<User>[] = [
		{
			accessorKey: "email",
			header: "Email",
			cell: ({ row }) => (
				<div className="flex items-center gap-2">
					<UserIcon className="h-4 w-4 text-muted-foreground" />
					<span className="font-medium">{row.original.email}</span>
				</div>
			),
		},
		{
			accessorKey: "name",
			header: "Name",
		},
		{
			accessorKey: "provider",
			header: "Provider",
			cell: ({ row }) => (
				<span className="capitalize">
					{row.original.provider || "internal"}
				</span>
			),
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
							onEditGroups(row.original);
						}}
						title="Edit Groups"
					>
						<UserCog className="h-4 w-4" />
					</Button>
				</div>
			),
		},
	];

	if (!isLoading && users.length === 0) {
		return (
			<Empty className="py-12">
				<EmptyHeader>
					<EmptyMedia variant="icon">
						<UserIcon />
					</EmptyMedia>
					<EmptyTitle>{query ? "No users found" : "No users"}</EmptyTitle>
					<EmptyDescription>
						{query
							? `No results for "${query}"`
							: "There are no users in the system yet."}
					</EmptyDescription>
				</EmptyHeader>
			</Empty>
		);
	}

	return <DataTable columns={columns} data={users} nameColumnWidth="w-[40%]" />;
}

import { timestampDate } from "@bufbuild/protobuf/wkt";
import type { ColumnDef } from "@tanstack/react-table";
import { format } from "date-fns";
import { User as UserIcon } from "lucide-react";
import { DataTable } from "@/components/data-table";
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
}

export function UserTable({ users, isLoading, query }: UserTableProps) {
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

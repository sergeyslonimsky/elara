import { timestampDate } from "@bufbuild/protobuf/wkt";
import type { ColumnDef } from "@tanstack/react-table";
import { format } from "date-fns";
import { User as UserIcon } from "lucide-react";
import { DataTable } from "@/components/data-table";
import { Badge } from "@/components/ui/badge";
import {
	Empty,
	EmptyDescription,
	EmptyHeader,
	EmptyMedia,
	EmptyTitle,
} from "@/components/ui/empty";
import { type User, UserStatus } from "@/gen/elara/user/v1/user_pb";

interface UserTableProps {
	users: User[];
	isLoading: boolean;
	query?: string;
	showDeactivated?: boolean;
	onRowClick: (user: User, event: React.MouseEvent) => void;
}

export function UserTable({
	users,
	isLoading,
	query,
	showDeactivated = false,
	onRowClick,
}: Readonly<UserTableProps>) {
	const visibleUsers = showDeactivated
		? users
		: users.filter((u) => u.status !== UserStatus.DEACTIVATED);

	const columns: ColumnDef<User>[] = [
		{
			accessorKey: "email",
			header: "Email",
			cell: ({ row }) => {
				const isDeactivated = row.original.status === UserStatus.DEACTIVATED;
				return (
					<div className="flex items-center gap-2">
						<UserIcon
							className={`h-4 w-4 ${isDeactivated ? "text-muted-foreground/50" : "text-muted-foreground"}`}
						/>
						<span
							className={`font-medium ${isDeactivated ? "text-muted-foreground" : ""}`}
						>
							{row.original.email}
						</span>
						{isDeactivated && <Badge variant="outline">Deactivated</Badge>}
					</div>
				);
			},
		},
		{
			accessorKey: "displayName",
			header: "Name",
		},
		{
			id: "provider",
			header: "Provider",
			cell: ({ row }) => {
				const provider = row.original.identities?.[0]?.provider || "internal";
				return <span className="capitalize">{provider}</span>;
			},
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

	if (!isLoading && visibleUsers.length === 0) {
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

	return (
		<DataTable
			columns={columns}
			data={visibleUsers}
			nameColumnWidth="w-[40%]"
			onRowClick={onRowClick}
		/>
	);
}

import { timestampDate } from "@bufbuild/protobuf/wkt";
import type { ColumnDef, SortingState } from "@tanstack/react-table";
import { format } from "date-fns";
import { Key } from "lucide-react";
import { DataTable } from "@/components/data-table";
import { SortableHeader } from "@/components/sortable-header";
import { Badge } from "@/components/ui/badge";
import {
	Empty,
	EmptyDescription,
	EmptyHeader,
	EmptyMedia,
	EmptyTitle,
} from "@/components/ui/empty";
import { PermissionAction } from "@/gen/elara/common/v1/permission_pb";
import type { Token } from "@/gen/elara/token/v1/token_pb";
import { timeAgo } from "@/lib/time";
import { RevokeTokenDialog } from "./revoke-token-dialog";

interface TokenTableProps {
	readonly tokens: Token[];
	readonly isLoading: boolean;
	readonly sorting: SortingState;
	readonly onSortingChange: (sorting: SortingState) => void;
	readonly hasFilters: boolean;
}

function formatPermission(action: PermissionAction): string {
	switch (action) {
		case PermissionAction.READ:
			return "read";
		case PermissionAction.WRITE:
			return "write";
		case PermissionAction.CREATE:
			return "create";
		case PermissionAction.ALL:
			return "all";
		default:
			return "—";
	}
}

function formatExpires(token: Token): string {
	if (!token.expiresAt) return "Never";
	const date = timestampDate(token.expiresAt);
	if (date.getTime() < Date.now()) return "Expired";
	return format(date, "PP");
}

function formatLastUsed(token: Token): string {
	if (!token.lastUsedAt) return "Never";
	return timeAgo(timestampDate(token.lastUsedAt));
}

// Tokens created before usernames existed may lack one; fall back to the name.
function displayName(token: Token): string {
	return token.username || token.name;
}

const columns: ColumnDef<Token>[] = [
	{
		// Column id "username" is also the server sort field: the list is
		// ordered by the full login shown here, not by the name prefix.
		accessorKey: "username",
		header: ({ column }) => (
			<SortableHeader column={column}>Username</SortableHeader>
		),
		cell: ({ row }) => (
			<div className="flex items-center gap-2">
				<Key className="h-4 w-4 shrink-0 text-muted-foreground" />
				<span className="font-mono font-medium text-sm">
					{displayName(row.original)}
				</span>
			</div>
		),
	},
	{
		accessorKey: "issuedBy",
		header: "Issued by",
		cell: ({ row }) => row.original.issuedBy || "—",
	},
	{
		accessorKey: "namespaces",
		header: "Namespaces",
		cell: ({ row }) => (
			<div className="flex flex-wrap gap-1">
				{row.original.namespaces.map((ns) => (
					<Badge key={ns} variant="secondary" className="font-mono text-xs">
						{ns}
					</Badge>
				))}
			</div>
		),
	},
	{
		accessorKey: "permission",
		header: "Permission",
		cell: ({ row }) => (
			<Badge variant="outline" className="uppercase">
				{formatPermission(row.original.permission)}
			</Badge>
		),
	},
	{
		accessorKey: "lastUsed",
		id: "last_used",
		header: ({ column }) => (
			<SortableHeader column={column}>Last used</SortableHeader>
		),
		cell: ({ row }) => (
			<span className="text-muted-foreground text-sm">
				{formatLastUsed(row.original)}
				{row.original.lastUsedIp ? ` · ${row.original.lastUsedIp}` : ""}
			</span>
		),
	},
	{
		accessorKey: "expiresAt",
		header: "Expires",
		cell: ({ row }) => (
			<span className="text-muted-foreground text-sm">
				{formatExpires(row.original)}
			</span>
		),
	},
	{
		accessorKey: "createdAt",
		id: "created",
		header: ({ column }) => (
			<SortableHeader column={column}>Created</SortableHeader>
		),
		cell: ({ row }) => {
			const date = row.original.createdAt
				? timestampDate(row.original.createdAt)
				: null;
			return date ? format(date, "PP") : "-";
		},
	},
	{
		id: "actions",
		header: () => <span className="sr-only">Actions</span>,
		cell: ({ row }) => (
			<div className="flex justify-end">
				<RevokeTokenDialog
					tokenId={row.original.id}
					tokenName={displayName(row.original)}
				/>
			</div>
		),
	},
];

export function TokenTable({
	tokens,
	isLoading,
	sorting,
	onSortingChange,
	hasFilters,
}: TokenTableProps) {
	if (!isLoading && tokens.length === 0) {
		return (
			<Empty className="py-12">
				<EmptyHeader>
					<EmptyMedia variant="icon">
						<Key />
					</EmptyMedia>
					<EmptyTitle>
						{hasFilters ? "No tokens found" : "No tokens"}
					</EmptyTitle>
					<EmptyDescription>
						{hasFilters
							? "Try adjusting your search or filters."
							: "Create an API token to grant programmatic access."}
					</EmptyDescription>
				</EmptyHeader>
			</Empty>
		);
	}

	return (
		<DataTable
			columns={columns}
			data={tokens}
			sorting={sorting}
			onSortingChange={onSortingChange}
			nameColumnWidth="w-[20%]"
		/>
	);
}

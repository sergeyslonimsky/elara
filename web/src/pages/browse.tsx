import { timestampDate } from "@bufbuild/protobuf/wkt";
import { useQuery } from "@connectrpc/connect-query";
import type { ColumnDef } from "@tanstack/react-table";
import { Database, FilePlus, Folder } from "lucide-react";
import { useEffect } from "react";
import { Link, useNavigate, useParams } from "react-router";
import { DataTable } from "@/components/data-table";
import { DirectoryTable } from "@/components/directory-table";
import { ErrorCard } from "@/components/error-card";
import { PageHeader } from "@/components/page-header";
import { PaginationControls } from "@/components/pagination-controls";
import { PathBreadcrumb } from "@/components/path-breadcrumb";
import { SearchInput } from "@/components/search-input";
import { SortableHeader } from "@/components/sortable-header";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
	Empty,
	EmptyContent,
	EmptyDescription,
	EmptyHeader,
	EmptyMedia,
	EmptyTitle,
} from "@/components/ui/empty";
import { Skeleton } from "@/components/ui/skeleton";
import { listConfigs } from "@/gen/elara/config/v1/config_service-ConfigService_connectquery";
import type { Namespace } from "@/gen/elara/namespace/v1/namespace_pb";
import { listNamespaces } from "@/gen/elara/namespace/v1/namespace_service-NamespaceService_connectquery";
import { useTableState } from "@/hooks/use-table-state";
import { timeAgo, tsToMs } from "@/lib/time";

const nsColumns: ColumnDef<Namespace>[] = [
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

function NamespaceList({
	tableState,
}: {
	tableState: ReturnType<typeof useTableState>;
}) {
	const navigate = useNavigate();
	const {
		offset,
		pageSize,
		sorting,
		searchInput,
		query,
		setOffset,
		setPageSize,
		setSearchInput,
		handleSearch,
		handleClear,
		sortParams,
		paginationParams,
		setSorting,
	} = tableState;

	const { data, isLoading, error, refetch, isFetching } = useQuery(
		listNamespaces,
		{
			pagination: paginationParams,
			sort: sortParams.field
				? { field: sortParams.field, direction: sortParams.direction }
				: undefined,
			query,
		},
	);

	const total = data?.pagination?.total ?? 0;

	return (
		<div className="flex flex-1 flex-col">
			<PageHeader
				title="Browse"
				onRefresh={() => void refetch()}
				isRefreshing={isFetching}
			>
				<SearchInput
					value={searchInput}
					onChange={setSearchInput}
					onSearch={handleSearch}
					onClear={handleClear}
					placeholder="Search namespaces..."
				/>
			</PageHeader>

			<div className="flex flex-1 flex-col gap-4 p-4">
				<div className="flex items-center">
					<PathBreadcrumb path="/" />
				</div>

				{error && <ErrorCard message={error.message} />}

				{isLoading ? (
					<div className="space-y-2">
						{Array.from({ length: 5 }).map((_, i) => (
							// biome-ignore lint/suspicious/noArrayIndexKey: skeleton
							<Skeleton key={i} className="h-10 w-full" />
						))}
					</div>
				) : total === 0 ? (
					<div className="py-16">
						<Empty>
							<EmptyHeader>
								<EmptyMedia variant="icon">
									<Database />
								</EmptyMedia>
								<EmptyTitle>
									{query ? "No namespaces found" : "No namespaces"}
								</EmptyTitle>
								<EmptyDescription>
									{query
										? `No results for "${query}"`
										: "Create a namespace to start managing configs"}
								</EmptyDescription>
							</EmptyHeader>
							{!query && (
								<EmptyContent>
									<Button
										variant="outline"
										size="sm"
										render={<Link to="/namespaces" />}
									>
										Go to Namespaces
									</Button>
								</EmptyContent>
							)}
						</Empty>
					</div>
				) : (
					<DataTable
						columns={nsColumns}
						data={data?.namespaces ?? []}
						sorting={sorting}
						onSortingChange={setSorting}
						onRowClick={(row) => navigate(`/browse/${row.name}`)}
					/>
				)}

				<PaginationControls
					total={total}
					pageSize={pageSize}
					offset={offset}
					onOffsetChange={setOffset}
					onPageSizeChange={setPageSize}
				/>
			</div>
		</div>
	);
}

export function BrowsePage() {
	const { namespace, "*": splat = "" } = useParams();
	const path = namespace ? `/${splat}` : undefined;
	const tableState = useTableState();

	const {
		offset,
		pageSize,
		sorting,
		searchInput,
		query,
		setOffset,
		setPageSize,
		setSorting,
		setSearchInput,
		handleSearch,
		handleClear,
		reset,
		sortParams,
		paginationParams,
	} = tableState;

	// biome-ignore lint/correctness/useExhaustiveDependencies: reset on route change
	useEffect(() => {
		reset();
	}, [namespace, path, reset]);

	const { data, isLoading, error, refetch, isFetching } = useQuery(
		listConfigs,
		{
			namespace: namespace ?? "",
			path: path ?? "/",
			pagination: paginationParams,
			sort: sortParams.field
				? { field: sortParams.field, direction: sortParams.direction }
				: undefined,
			query,
		},
		{ enabled: !!namespace },
	);

	const total = data?.pagination?.total ?? 0;

	if (!namespace) {
		return <NamespaceList tableState={tableState} />;
	}

	const newConfigPath =
		path === "/"
			? `/config/new/${namespace}`
			: `/config/new/${namespace}${path}`;

	return (
		<div className="flex flex-1 flex-col">
			<PageHeader
				title="Browse"
				onRefresh={() => void refetch()}
				isRefreshing={isFetching}
			>
				<SearchInput
					value={searchInput}
					onChange={setSearchInput}
					onSearch={handleSearch}
					onClear={handleClear}
					placeholder="Search configs..."
				/>
			</PageHeader>

			<div className="flex flex-1 flex-col gap-4 p-4">
				<div className="flex items-center justify-between">
					<PathBreadcrumb namespace={namespace} path={path ?? "/"} />
					<Button size="sm" render={<Link to={newConfigPath} />}>
						<FilePlus className="mr-1 h-4 w-4" />
						New Config
					</Button>
				</div>

				{error && <ErrorCard message={error.message} />}

				<DirectoryTable
					namespace={namespace}
					currentPath={path ?? "/"}
					entries={data?.entries ?? []}
					isLoading={isLoading}
					sorting={sorting}
					onSortingChange={setSorting}
				/>

				<PaginationControls
					total={total}
					pageSize={pageSize}
					offset={offset}
					onOffsetChange={setOffset}
					onPageSizeChange={setPageSize}
				/>
			</div>
		</div>
	);
}

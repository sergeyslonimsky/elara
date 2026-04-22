import { useQuery } from "@connectrpc/connect-query";
import { Database } from "lucide-react";
import { Link, useNavigate } from "react-router";
import { DataTable } from "@/components/data-table";
import { ErrorCard } from "@/components/error-card";
import { PageShell } from "@/components/page-shell";
import { PaginationControls } from "@/components/pagination-controls";
import { PathBreadcrumb } from "@/components/path-breadcrumb";
import { SearchInput } from "@/components/search-input";
import { SkeletonList } from "@/components/skeleton-list";
import { Button } from "@/components/ui/button";
import {
	Empty,
	EmptyContent,
	EmptyDescription,
	EmptyHeader,
	EmptyMedia,
	EmptyTitle,
} from "@/components/ui/empty";
import { listNamespaces } from "@/gen/elara/namespace/v1/namespace_service-NamespaceService_connectquery";
import type { useTableState } from "@/hooks/use-table-state";
import { nsColumns } from "./ns-columns";

export function NamespaceList({
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
		<PageShell
			title="Browse"
			onRefresh={() => void refetch()}
			isRefreshing={isFetching}
			headerSlot={
				<SearchInput
					value={searchInput}
					onChange={setSearchInput}
					onSearch={handleSearch}
					onClear={handleClear}
					placeholder="Search namespaces..."
				/>
			}
		>
			<div className="flex items-center">
				<PathBreadcrumb path="/" />
			</div>

			{error && <ErrorCard message={error.message} />}

			{isLoading ? (
				<SkeletonList count={5} />
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
		</PageShell>
	);
}

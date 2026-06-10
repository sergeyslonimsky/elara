import { useQuery } from "@connectrpc/connect-query";
import type { SortingState } from "@tanstack/react-table";
import { useMemo } from "react";
import { ErrorCard } from "@/components/error-card";
import { PageShell } from "@/components/page-shell";
import { PaginationControls } from "@/components/pagination-controls";
import { SearchInput } from "@/components/search-input";
import { SkeletonList } from "@/components/skeleton-list";
import { SortDirection } from "@/gen/elara/common/v1/common_pb";
import { listTokens } from "@/gen/elara/token/v1/token_service-TokenService_connectquery";
import { useTableState } from "@/hooks/use-table-state";
import { CreateTokenDialog } from "./components/create-token-dialog";
import { TokenTable } from "./components/token-table";

const SORTABLE_FIELDS = new Set(["name", "last_used", "created"]);

function sortingToServerParams(sorting: SortingState) {
	if (sorting.length === 0) {
		return { field: "", direction: SortDirection.UNSPECIFIED };
	}
	const s = sorting[0];
	const field = SORTABLE_FIELDS.has(s.id) ? s.id : "";
	if (!field) {
		return { field: "", direction: SortDirection.UNSPECIFIED };
	}
	return {
		field,
		direction: s.desc ? SortDirection.DESC : SortDirection.ASC,
	};
}

export function TokensPage() {
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
	} = useTableState({ initialSorting: [{ id: "created", desc: true }] });

	const sortParams = useMemo(() => sortingToServerParams(sorting), [sorting]);

	const { data, isLoading, error, refetch, isFetching } = useQuery(listTokens, {
		pagination: { limit: pageSize, offset },
		sorting: sortParams.field
			? { field: sortParams.field, direction: sortParams.direction }
			: undefined,
		filters: query
			? { queryParams: [query], issuedBy: [], namespaces: [] }
			: undefined,
	});

	const tokens = data?.tokens ?? [];
	const total = data?.pagination?.total ?? 0;
	const hasFilters = query.length > 0;

	return (
		<PageShell
			title="Tokens"
			onRefresh={() => refetch()}
			isRefreshing={isFetching}
			headerSlot={
				<div className="flex items-center gap-2">
					<SearchInput
						value={searchInput}
						onChange={setSearchInput}
						onSearch={handleSearch}
						onClear={handleClear}
						placeholder="Search tokens..."
					/>
					<CreateTokenDialog />
				</div>
			}
		>
			<div className="flex flex-col gap-4">
				{error && <ErrorCard message={error.message} />}

				{isLoading ? (
					<SkeletonList count={5} className="h-16" />
				) : (
					!error && (
						<TokenTable
							tokens={tokens}
							isLoading={isLoading}
							sorting={sorting}
							onSortingChange={setSorting}
							hasFilters={hasFilters}
						/>
					)
				)}

				<PaginationControls
					total={total}
					pageSize={pageSize}
					offset={offset}
					onOffsetChange={setOffset}
					onPageSizeChange={setPageSize}
				/>
			</div>
		</PageShell>
	);
}

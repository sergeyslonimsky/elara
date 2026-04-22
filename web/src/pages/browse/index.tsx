import { useQuery } from "@connectrpc/connect-query";
import { FilePlus } from "lucide-react";
import { useEffect } from "react";
import { useParams } from "react-router";
import { DirectoryTable } from "@/components/directory-table";
import { ErrorCard } from "@/components/error-card";
import { LockAwareButton } from "@/components/lock-aware-button";
import { PageShell } from "@/components/page-shell";
import { PaginationControls } from "@/components/pagination-controls";
import { PathBreadcrumb } from "@/components/path-breadcrumb";
import { SearchInput } from "@/components/search-input";
import { listConfigs } from "@/gen/elara/config/v1/config_service-ConfigService_connectquery";
import { getNamespace } from "@/gen/elara/namespace/v1/namespace_service-NamespaceService_connectquery";
import { useTableState } from "@/hooks/use-table-state";
import { NamespaceList } from "./namespace-list";

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
			namespace: namespace,
			path: path ?? "/",
			pagination: paginationParams,
			sort: sortParams.field
				? { field: sortParams.field, direction: sortParams.direction }
				: undefined,
			query,
		},
		{ enabled: !!namespace },
	);

	const { data: namespaceData } = useQuery(
		getNamespace,
		{ name: namespace },
		{ enabled: !!namespace },
	);

	const namespaceLocked = namespaceData?.namespace?.locked ?? false;

	const total = data?.pagination?.total ?? 0;

	if (!namespace) {
		return <NamespaceList tableState={tableState} />;
	}

	const newConfigPath =
		path === "/"
			? `/config/new/${namespace}`
			: `/config/new/${namespace}${path}`;

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
					placeholder="Search configs..."
				/>
			}
		>
			<div className="flex items-center justify-between">
				<PathBreadcrumb namespace={namespace} path={path ?? "/"} />
				<LockAwareButton
					size="sm"
					locked={namespaceLocked}
					lockedReason={`Namespace "${namespace}" is locked`}
					to={newConfigPath}
				>
					<FilePlus className="mr-1 h-4 w-4" />
					New Config
				</LockAwareButton>
			</div>

			{error && <ErrorCard message={error.message} />}

			<DirectoryTable
				namespace={namespace}
				currentPath={path ?? "/"}
				entries={data?.entries ?? []}
				isLoading={isLoading}
				sorting={sorting}
				onSortingChange={setSorting}
				namespaceLocked={namespaceLocked}
			/>

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

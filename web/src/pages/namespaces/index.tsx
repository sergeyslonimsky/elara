import { useQuery } from "@connectrpc/connect-query";
import { Database } from "lucide-react";
import { ErrorCard } from "@/components/error-card";
import { ExportDialog } from "@/components/export-dialog";
import { ImportDialog } from "@/components/import-dialog";
import { PageShell } from "@/components/page-shell";
import { SearchInput } from "@/components/search-input";
import { SkeletonList } from "@/components/skeleton-list";
import {
	Empty,
	EmptyDescription,
	EmptyHeader,
	EmptyMedia,
	EmptyTitle,
} from "@/components/ui/empty";
import { listNamespaces } from "@/gen/elara/namespace/v1/namespace_service-NamespaceService_connectquery";
import { useSearchInput } from "@/hooks/use-search-input";
import { CreateDialog } from "./create-dialog";
import { NamespaceCard } from "./namespace-card";

export function NamespacesPage() {
	const { searchInput, setSearchInput, query, handleSearch, handleClear } =
		useSearchInput();
	const { data, isLoading, error, refetch, isFetching } = useQuery(
		listNamespaces,
		{ query },
	);

	return (
		<PageShell
			title="Namespaces"
			onRefresh={() => {
				refetch();
			}}
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
			<div className="flex items-center justify-end gap-2">
				<ImportDialog />
				<ExportDialog />
				<CreateDialog />
			</div>

			{error && <ErrorCard message={error.message} />}

			<div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
				{isLoading && (
					<SkeletonList
						count={3}
						className="h-32 rounded-xl"
						wrapperClassName="contents"
					/>
				)}

				{data?.namespaces.map((ns) => (
					<NamespaceCard key={ns.name} ns={ns} />
				))}

				{data?.namespaces.length === 0 && (
					<div className="col-span-full py-16">
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
										: "Create your first namespace to get started"}
								</EmptyDescription>
							</EmptyHeader>
						</Empty>
					</div>
				)}
			</div>
		</PageShell>
	);
}

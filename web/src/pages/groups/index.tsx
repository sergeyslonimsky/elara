import { useQuery } from "@connectrpc/connect-query";
import { Plus } from "lucide-react";
import { useState } from "react";
import { useNavigate } from "react-router";
import { Can } from "@/auth/ability-context";
import { ErrorCard } from "@/components/error-card";
import { PageShell } from "@/components/page-shell";
import { PaginationControls } from "@/components/pagination-controls";
import { SearchInput } from "@/components/search-input";
import { SkeletonList } from "@/components/skeleton-list";
import { Button } from "@/components/ui/button";
import type { Group } from "@/gen/elara/group/v1/group_pb";
import { listGroups } from "@/gen/elara/group/v1/group_service-GroupService_connectquery";
import { useTableState } from "@/hooks/use-table-state";
import {
	CreateGroupDialog,
	DeleteGroupDialog,
} from "./components/group-dialogs";
import { GroupTable } from "./components/group-table";

export function GroupsPage() {
	const navigate = useNavigate();
	const {
		offset,
		pageSize,
		searchInput,
		query,
		setOffset,
		setPageSize,
		setSearchInput,
		handleSearch,
		handleClear,
	} = useTableState();

	const [isCreateOpen, setIsCreateOpen] = useState(false);
	const [deletingGroup, setDeletingGroup] = useState<Group | null>(null);

	const { data, isLoading, error, refetch, isFetching } = useQuery(listGroups, {
		pagination: { limit: pageSize, offset },
		search: query,
	});

	return (
		<PageShell
			title="Groups"
			onRefresh={() => refetch()}
			isRefreshing={isFetching}
			headerSlot={
				<SearchInput
					value={searchInput}
					onChange={setSearchInput}
					onSearch={handleSearch}
					onClear={handleClear}
					placeholder="Search groups..."
				/>
			}
		>
			<div className="flex flex-col gap-4">
				<div className="flex justify-end">
					<Can I="write" a="Group">
						{() => (
							<Button size="sm" onClick={() => setIsCreateOpen(true)}>
								<Plus className="mr-1 h-4 w-4" />
								New Group
							</Button>
						)}
					</Can>
				</div>

				{error && <ErrorCard message={error.message} />}

				{isLoading ? (
					<SkeletonList count={5} className="h-16" />
				) : (
					<GroupTable
						groups={data?.groups ?? []}
						isLoading={isLoading}
						query={query}
						onRowClick={(group) => navigate(`/groups/${group.name}`)}
						onDelete={setDeletingGroup}
					/>
				)}

				<PaginationControls
					total={data?.pagination?.total ?? 0}
					pageSize={pageSize}
					offset={offset}
					onOffsetChange={setOffset}
					onPageSizeChange={setPageSize}
				/>
			</div>

			<CreateGroupDialog open={isCreateOpen} onOpenChange={setIsCreateOpen} />

			<DeleteGroupDialog
				key={deletingGroup?.name ?? "none"}
				group={deletingGroup}
				open={!!deletingGroup}
				onOpenChange={(open) => !open && setDeletingGroup(null)}
			/>
		</PageShell>
	);
}

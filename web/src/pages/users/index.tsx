import { useQuery } from "@connectrpc/connect-query";
import { useState } from "react";
import { useNavigate } from "react-router";
import { ErrorCard } from "@/components/error-card";
import { PageShell } from "@/components/page-shell";
import { PaginationControls } from "@/components/pagination-controls";
import { SearchInput } from "@/components/search-input";
import { SkeletonList } from "@/components/skeleton-list";
import { Checkbox } from "@/components/ui/checkbox";
import { Label } from "@/components/ui/label";
import { listUsers } from "@/gen/elara/user/v1/user_service-UserService_connectquery";
import { useTableState } from "@/hooks/use-table-state";
import { CreateUserDialog } from "./components/create-user-dialog";
import { UserTable } from "./components/user-table";

export function UsersPage() {
	const navigate = useNavigate();
	const [showDeactivated, setShowDeactivated] = useState(false);
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

	const { data, isLoading, error, refetch, isFetching } = useQuery(listUsers, {
		pagination: { limit: pageSize, offset },
		search: query,
	});

	return (
		<PageShell
			title="Users"
			onRefresh={() => refetch()}
			isRefreshing={isFetching}
			headerSlot={
				<div className="flex items-center gap-2">
					<SearchInput
						value={searchInput}
						onChange={setSearchInput}
						onSearch={handleSearch}
						onClear={handleClear}
						placeholder="Search users..."
					/>
					<CreateUserDialog />
				</div>
			}
		>
			<div className="flex flex-col gap-4">
				{error && <ErrorCard message={error.message} />}

				<div className="flex items-center gap-2">
					<Checkbox
						id="show-deactivated"
						checked={showDeactivated}
						onCheckedChange={(checked) => setShowDeactivated(checked === true)}
					/>
					<Label htmlFor="show-deactivated" className="cursor-pointer">
						Show deactivated
					</Label>
				</div>

				{isLoading ? (
					<SkeletonList count={5} className="h-16" />
				) : (
					<UserTable
						users={data?.users ?? []}
						isLoading={isLoading}
						query={query}
						showDeactivated={showDeactivated}
						onRowClick={(user) => {
							navigate(`/users/${user.id}`);
						}}
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
		</PageShell>
	);
}

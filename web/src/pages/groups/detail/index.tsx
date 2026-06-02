import { useQuery } from "@connectrpc/connect-query";
import { useEffect } from "react";
import { useParams } from "react-router";
import { BackButton } from "@/components/back-button";
import { ErrorCard } from "@/components/error-card";
import { PageShell } from "@/components/page-shell";
import { Skeleton } from "@/components/ui/skeleton";
import { getGroup } from "@/gen/elara/group/v1/group_service-GroupService_connectquery";
import { GroupDetailHeader } from "./group-detail-header";
import { GroupDetailTabs } from "./group-detail-tabs";

export function GroupDetailPage() {
	const { id = "" } = useParams();

	const { data, isLoading, error } = useQuery(
		getGroup,
		{ name: id },
		{ enabled: !!id },
	);

	useEffect(() => {
		if (!data?.group) return;
		document.title = `${data.group.displayName || data.group.name} • Elara`;
		return () => {
			document.title = "Elara";
		};
	}, [data?.group]);

	if (isLoading) {
		return (
			<PageShell title="Group Detail">
				<BackButton to="/groups" label="Back to groups" />
				<Skeleton className="h-24 w-full rounded-xl" />
				<Skeleton className="h-64 w-full rounded-xl" />
			</PageShell>
		);
	}

	if (error || !data?.group) {
		return (
			<PageShell title="Group Detail">
				<BackButton to="/groups" label="Back to groups" />
				<ErrorCard message={error?.message ?? "Group not found"} />
			</PageShell>
		);
	}

	return (
		<PageShell title={data.group.displayName || data.group.name}>
			<BackButton to="/groups" label="Back to groups" />
			<GroupDetailHeader group={data.group} />
			<GroupDetailTabs
				group={data.group}
				visibleMembers={data.visibleMembers}
				permissions={data.permissions}
			/>
		</PageShell>
	);
}

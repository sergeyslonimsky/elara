import { useQuery } from "@connectrpc/connect-query";
import { useEffect } from "react";
import { useParams } from "react-router";
import { BackButton } from "@/components/back-button";
import { ErrorCard } from "@/components/error-card";
import { PageShell } from "@/components/page-shell";
import { Skeleton } from "@/components/ui/skeleton";
import { getUser } from "@/gen/elara/user/v1/user_service-UserService_connectquery";
import { UserDetailHeader } from "./user-detail-header";
import { UserDetailTabs } from "./user-detail-tabs";

export function UserDetailPage() {
	const { userId = "" } = useParams();

	const { data, isLoading, error, refetch } = useQuery(
		getUser,
		{ userId },
		{ enabled: !!userId },
	);

	useEffect(() => {
		if (!data?.user) return;
		const base = data.user.displayName || data.user.email;
		document.title = `${base} • Elara`;
		return () => {
			document.title = "Elara";
		};
	}, [data?.user]);

	if (isLoading) {
		return (
			<PageShell title="User Detail">
				<BackButton to="/users" label="Back to users" />
				<Skeleton className="h-24 w-full rounded-xl" />
				<Skeleton className="h-64 w-full rounded-xl" />
			</PageShell>
		);
	}

	if (error || !data?.user) {
		return (
			<PageShell title="User Detail">
				<BackButton to="/users" label="Back to users" />
				<ErrorCard message={error?.message ?? "User not found"} />
			</PageShell>
		);
	}

	return (
		<PageShell title={data.user.displayName || data.user.email}>
			<BackButton to="/users" label="Back to users" />
			<UserDetailHeader user={data.user} onRefetch={() => refetch()} />
			<UserDetailTabs
				user={data.user}
				visibleGroupIds={data.visibleGroupIds}
				membershipVersion={data.membershipVersion}
			/>
		</PageShell>
	);
}

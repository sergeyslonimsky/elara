import { useQuery } from "@connectrpc/connect-query";
import { useCallback } from "react";
import { ErrorBoundary } from "react-error-boundary";
import { ErrorCard } from "@/components/error-card";
import { KpiCard } from "@/components/kpi-card";
import { PageShell } from "@/components/page-shell";
import { Skeleton } from "@/components/ui/skeleton";
import {
	getStats,
	listActivity,
} from "@/gen/elara/dashboard/v1/dashboard_service-DashboardService_connectquery";
import { listNamespaces } from "@/gen/elara/namespace/v1/namespace_service-NamespaceService_connectquery";
import { ActivityCard } from "./components/activity-card";
import { NamespacesCard } from "./components/namespaces-card";

const DEFAULT_ACTIVITIES_COUNT = 20;

export function DashboardPage() {
	const statsQ = useQuery(getStats, {}, { refetchInterval: 30_000 });
	const activityQ = useQuery(
		listActivity,
		{ limit: DEFAULT_ACTIVITIES_COUNT },
		{ refetchInterval: 30_000 },
	);
	const namespacesQ = useQuery(listNamespaces, {});

	const isRefreshing =
		statsQ.isFetching || activityQ.isFetching || namespacesQ.isFetching;

	const refresh = useCallback(() => {
		void statsQ.refetch();
		void activityQ.refetch();
		void namespacesQ.refetch();
	}, [statsQ.refetch, activityQ.refetch, namespacesQ.refetch]);

	return (
		<PageShell
			title="Dashboard"
			onRefresh={refresh}
			isRefreshing={isRefreshing}
			contentClassName="flex flex-1 flex-col gap-6 p-4"
		>
			<ErrorBoundary
				fallback={<ErrorCard message="Failed to load dashboard data" />}
			>
				{statsQ.error && <ErrorCard message={statsQ.error.message} />}
				{activityQ.error && <ErrorCard message={activityQ.error.message} />}
				{namespacesQ.error && <ErrorCard message={namespacesQ.error.message} />}

				<div className="grid grid-cols-2 gap-4 md:grid-cols-4">
					<KpiCard
						label="Namespaces"
						value={
							statsQ.isLoading ? (
								<Skeleton className="h-8 w-16" />
							) : (
								(statsQ.data?.namespaceCount ?? "—")
							)
						}
					/>
					<KpiCard
						label="Configs"
						value={
							statsQ.isLoading ? (
								<Skeleton className="h-8 w-16" />
							) : (
								(statsQ.data?.configCount ?? "—")
							)
						}
					/>
					<KpiCard
						label="Active Clients"
						value={
							statsQ.isLoading ? (
								<Skeleton className="h-8 w-16" />
							) : (
								(statsQ.data?.activeClientCount ?? "—")
							)
						}
						accentClass="text-emerald-500"
					/>
					<KpiCard
						label="Global Revision"
						value={
							statsQ.isLoading ? (
								<Skeleton className="h-8 w-24" />
							) : statsQ.data ? (
								statsQ.data.globalRevision.toLocaleString()
							) : (
								"—"
							)
						}
						accentClass="text-blue-500"
					/>
				</div>

				<div className="grid gap-4 lg:grid-cols-3">
					<ActivityCard
						entries={activityQ.data?.entries}
						isLoading={activityQ.isLoading}
						limit={DEFAULT_ACTIVITIES_COUNT}
					/>
					<NamespacesCard
						namespaces={namespacesQ.data?.namespaces}
						isLoading={namespacesQ.isLoading}
					/>
				</div>
			</ErrorBoundary>
		</PageShell>
	);
}

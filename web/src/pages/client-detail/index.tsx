import { useQuery } from "@connectrpc/connect-query";
import { ArrowLeft } from "lucide-react";
import { useEffect, useMemo } from "react";
import { Link, useParams } from "react-router";
import { AppHeader } from "@/components/app-header";
import { ErrorCard } from "@/components/error-card";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { getClient } from "@/gen/elara/clients/v1/clients_service-ClientsService_connectquery";
import { useWatchClient } from "@/hooks/use-watch-client";
import { tsToMs } from "@/lib/time";
import { DetailHeader } from "./detail-header";
import { DetailTabs } from "./detail-tabs";
import { KpiRow } from "./kpi-row";

export function ClientDetailPage() {
	const { id = "" } = useParams();

	// Initial unary fetch — instant first paint
	const initialQ = useQuery(getClient, { id }, { enabled: !!id, staleTime: 0 });

	// Live stream — overrides snapshot when connected
	const live = useWatchClient(id);

	const client = live.snapshot ?? initialQ.data?.client;
	const isActive = client ? !client.disconnectedAt : false;

	// Activity: live events prepended to the initial server-side ring buffer.
	const initialEvents = initialQ.data?.recentEvents ?? [];
	const liveEvents = live.events;
	const events = useMemo(() => {
		// liveEvents are newest-first; initialEvents are oldest-first
		// (server returns chronological). Merge so newest is on top.
		const initialReversed = [...initialEvents].reverse();
		// Avoid duplication: if liveEvents already contains anything,
		// trust live as the source of truth past the merge boundary.
		if (liveEvents.length === 0) return initialReversed;
		const initialMs = initialReversed[0]
			? tsToMs(initialReversed[0].timestamp)
			: 0;
		const newOnly = liveEvents.filter((e) => tsToMs(e.timestamp) > initialMs);
		return [...newOnly, ...initialReversed];
	}, [liveEvents, initialEvents]);

	// Tab title: prefix with (disconnected) when client is gone.
	useEffect(() => {
		if (!client) return;
		const base = client.clientName || `${client.peerAddress}` || "client";
		document.title = isActive
			? `${base} • Elara`
			: `(disconnected) ${base} • Elara`;
		return () => {
			document.title = "Elara";
		};
	}, [client, isActive]);

	if (initialQ.isLoading && !client) {
		return (
			<>
				<AppHeader />
				<div className="flex flex-1 flex-col gap-4 p-4 pt-0">
					<Skeleton className="mt-4 h-8 w-48" />
					<Skeleton className="h-32 w-full rounded-xl" />
					<Skeleton className="h-64 w-full rounded-xl" />
				</div>
			</>
		);
	}

	if (initialQ.error || !client) {
		return (
			<>
				<AppHeader />
				<div className="flex flex-1 flex-col gap-4 p-4 pt-0">
					<div className="mt-4">
						<Button variant="ghost" size="sm" render={<Link to="/clients" />}>
							<ArrowLeft className="mr-1 h-4 w-4" />
							Back to clients
						</Button>
					</div>
					<ErrorCard message={initialQ.error?.message ?? "Client not found"} />
				</div>
			</>
		);
	}

	return (
		<>
			<AppHeader />
			<div className="flex flex-1 flex-col gap-4 p-4 pt-0">
				<div className="mt-4">
					<Button variant="ghost" size="sm" render={<Link to="/clients" />}>
						<ArrowLeft className="mr-1 h-4 w-4" />
						Back to clients
					</Button>
				</div>
				<DetailHeader
					client={client}
					isActive={isActive}
					streamStatus={live.status}
				/>
				<KpiRow client={client} isActive={isActive} />
				<DetailTabs client={client} events={events} isActive={isActive} />
			</div>
		</>
	);
}

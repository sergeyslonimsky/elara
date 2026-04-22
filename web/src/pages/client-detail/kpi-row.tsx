import { KpiCard } from "@/components/kpi-card";
import type { Client } from "@/gen/elara/clients/v1/clients_pb";
import { useHistory } from "@/hooks/use-history";
import { totalRequests } from "@/lib/client";
import { formatUptime } from "@/lib/duration";
import { timeAgo, tsToMs } from "@/lib/time";

export function KpiRow({
	client,
	isActive,
}: {
	client: Client;
	isActive: boolean;
}) {
	const requests = totalRequests(client);
	const errors = Number(client.errorCount);
	const watches = client.activeWatches;
	const requestSeries = useHistory(requests);
	const errorSeries = useHistory(errors);
	const watchSeries = useHistory(watches);

	const connectedAtMs = tsToMs(client.connectedAt);
	const disconnectedAtMs = client.disconnectedAt
		? tsToMs(client.disconnectedAt)
		: 0;
	const uptime = formatUptime(
		connectedAtMs,
		isActive ? Date.now() : disconnectedAtMs,
	);

	const lastActivityMs = tsToMs(client.lastActivityAt);
	const lastActivity = lastActivityMs ? timeAgo(new Date(lastActivityMs)) : "—";

	return (
		<div className="grid grid-cols-2 gap-3 md:grid-cols-3 xl:grid-cols-5">
			<KpiCard
				label="Requests"
				value={requests.toLocaleString()}
				series={requestSeries}
				accentClass="text-blue-500"
			/>
			<KpiCard
				label="Errors"
				value={errors.toLocaleString()}
				series={errorSeries}
				accentClass={errors > 0 ? "text-destructive" : "text-muted-foreground"}
			/>
			<KpiCard
				label="Active watches"
				value={watches}
				series={watchSeries}
				accentClass="text-violet-500"
			/>
			<KpiCard
				label="Uptime"
				value={uptime}
				subtitle={
					connectedAtMs
						? `since ${new Date(connectedAtMs).toLocaleString()}`
						: undefined
				}
			/>
			<KpiCard
				label="Last activity"
				value={lastActivity}
				subtitle={isActive ? "live" : undefined}
			/>
		</div>
	);
}

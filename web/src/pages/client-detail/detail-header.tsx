import { Badge } from "@/components/ui/badge";
import { Card, CardContent } from "@/components/ui/card";
import type { Client } from "@/gen/elara/clients/v1/clients_pb";
import type { StreamStatus } from "@/hooks/use-watch-client";
import { isLive } from "@/lib/duration";
import { timeAgo, tsToMs } from "@/lib/time";

interface DetailHeaderProps {
	client: Client;
	isActive: boolean;
	streamStatus: StreamStatus;
}

export function DetailHeader({
	client,
	isActive,
	streamStatus,
}: DetailHeaderProps) {
	const live = isLive(tsToMs(client.lastActivityAt));
	return (
		<Card className="rounded-xl">
			<CardContent className="space-y-2 pt-4">
				<div className="flex flex-wrap items-center gap-3">
					{isActive ? (
						<Badge
							variant="default"
							className={
								live
									? "bg-emerald-500 hover:bg-emerald-500"
									: "bg-muted-foreground hover:bg-muted-foreground"
							}
						>
							{live ? "● Active" : "● Idle"}
						</Badge>
					) : (
						<Badge variant="destructive">● Disconnected</Badge>
					)}
					<h1 className="font-semibold text-xl">
						{client.clientName || (
							<span className="text-muted-foreground italic">unknown</span>
						)}
					</h1>
					{client.clientVersion && (
						<Badge variant="outline">v{client.clientVersion}</Badge>
					)}
					{!isActive && client.disconnectedAt && (
						<span className="text-muted-foreground text-xs">
							disconnected {timeAgo(new Date(tsToMs(client.disconnectedAt)))}
						</span>
					)}
					{isActive && (
						<span className="ml-auto text-muted-foreground text-xs">
							stream: {streamStatus}
						</span>
					)}
				</div>
				<div className="flex flex-wrap gap-x-4 gap-y-1 text-muted-foreground text-xs">
					<span>
						peer <span className="font-mono">{client.peerAddress}</span>
					</span>
					{client.userAgent && <span>ua {client.userAgent}</span>}
					{client.k8sNamespace && (
						<span>
							ns <span className="font-mono">{client.k8sNamespace}</span>
						</span>
					)}
					{client.k8sPod && (
						<span>
							pod <span className="font-mono">{client.k8sPod}</span>
						</span>
					)}
					{client.k8sNode && (
						<span>
							node <span className="font-mono">{client.k8sNode}</span>
						</span>
					)}
					{client.instanceId && (
						<span>
							instance <span className="font-mono">{client.instanceId}</span>
						</span>
					)}
				</div>
			</CardContent>
		</Card>
	);
}

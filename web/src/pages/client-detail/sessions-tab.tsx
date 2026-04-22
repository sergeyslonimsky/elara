import { useQuery } from "@connectrpc/connect-query";
import { History as HistoryIcon } from "lucide-react";
import { useNavigate } from "react-router";
import { SkeletonList } from "@/components/skeleton-list";
import {
	Empty,
	EmptyDescription,
	EmptyHeader,
	EmptyMedia,
	EmptyTitle,
} from "@/components/ui/empty";
import {
	Table,
	TableBody,
	TableCell,
	TableHead,
	TableHeader,
	TableRow,
} from "@/components/ui/table";
import type { Client } from "@/gen/elara/clients/v1/clients_pb";
import { listClientSessions } from "@/gen/elara/clients/v1/clients_service-ClientsService_connectquery";
import { totalRequests } from "@/lib/client";
import { formatUptime } from "@/lib/duration";
import { timeAgo, tsToMs } from "@/lib/time";

export function SessionsTab({ client }: { client: Client }) {
	const navigate = useNavigate();
	const enabled = !!client.clientName;

	const q = useQuery(
		listClientSessions,
		{
			clientName: client.clientName,
			k8sNamespace: client.k8sNamespace,
			currentId: client.id,
			limit: 50,
		},
		{ enabled },
	);

	if (!enabled) {
		return (
			<Empty>
				<EmptyHeader>
					<EmptyMedia variant="icon">
						<HistoryIcon />
					</EmptyMedia>
					<EmptyTitle>Anonymous client</EmptyTitle>
					<EmptyDescription>
						Sessions can only be correlated for clients that send the{" "}
						<code className="rounded bg-muted px-1 py-0.5 text-xs">
							x-client-name
						</code>{" "}
						metadata header.
					</EmptyDescription>
				</EmptyHeader>
			</Empty>
		);
	}

	if (q.isLoading) {
		return <SkeletonList count={3} className="h-10 w-full rounded-lg" />;
	}

	const sessions = q.data?.sessions ?? [];
	if (sessions.length === 0) {
		return (
			<Empty>
				<EmptyHeader>
					<EmptyMedia variant="icon">
						<HistoryIcon />
					</EmptyMedia>
					<EmptyTitle>No previous sessions</EmptyTitle>
					<EmptyDescription>
						This is the first connection from{" "}
						<span className="font-mono">{client.clientName}</span>
						{client.k8sNamespace ? (
							<>
								{" "}
								in namespace{" "}
								<span className="font-mono">{client.k8sNamespace}</span>
							</>
						) : null}
						.
					</EmptyDescription>
				</EmptyHeader>
			</Empty>
		);
	}

	return (
		<Table>
			<TableHeader>
				<TableRow>
					<TableHead>Pod</TableHead>
					<TableHead>Disconnected</TableHead>
					<TableHead className="text-right">Duration</TableHead>
					<TableHead className="text-right">Requests</TableHead>
					<TableHead className="text-right">Errors</TableHead>
				</TableRow>
			</TableHeader>
			<TableBody>
				{sessions.map((s) => {
					const start = tsToMs(s.connectedAt);
					const end = tsToMs(s.disconnectedAt);
					const duration = start && end ? formatUptime(start, end) : "—";
					return (
						<TableRow
							key={s.id}
							className="cursor-pointer hover:bg-muted/50"
							onClick={() => navigate(`/clients/${s.id}`)}
						>
							<TableCell className="font-mono text-xs">
								{s.k8sPod || "—"}
							</TableCell>
							<TableCell className="text-xs">
								{end ? timeAgo(new Date(end)) : "—"}
							</TableCell>
							<TableCell className="text-right text-xs">{duration}</TableCell>
							<TableCell className="text-right text-xs">
								{totalRequests(s).toLocaleString()}
							</TableCell>
							<TableCell className="text-right text-xs">
								<span
									className={
										Number(s.errorCount) > 0
											? "font-medium text-destructive"
											: "text-muted-foreground"
									}
								>
									{Number(s.errorCount)}
								</span>
							</TableCell>
						</TableRow>
					);
				})}
			</TableBody>
		</Table>
	);
}

import { Link } from "react-router";
import { EventTypeBadge } from "@/components/event-type-badge";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import {
	Table,
	TableBody,
	TableCell,
	TableHead,
	TableHeader,
	TableRow,
} from "@/components/ui/table";
import { EventType } from "@/gen/elara/config/v1/config_pb";
import type { ActivityEntry } from "@/gen/elara/dashboard/v1/dashboard_service_pb";
import { timeAgo, tsToMs } from "@/lib/time";

const SKELETON_KEYS = ["a1", "a2", "a3", "a4", "a5", "a6"];

interface ActivityCardProps {
	entries: ActivityEntry[] | undefined;
	isLoading: boolean;
	limit: number;
}

export function ActivityCard({ entries, isLoading, limit }: ActivityCardProps) {
	return (
		<Card className="rounded-xl lg:col-span-2">
			<CardHeader className="pb-3">
				<CardTitle className="text-base">Last {limit} Changes</CardTitle>
			</CardHeader>
			<CardContent className="p-0">
				<Table>
					<TableHeader>
						<TableRow>
							<TableHead className="w-28">Type</TableHead>
							<TableHead>Namespace</TableHead>
							<TableHead>Path</TableHead>
							<TableHead className="w-28 text-right">When</TableHead>
						</TableRow>
					</TableHeader>
					<TableBody>
						{isLoading &&
							SKELETON_KEYS.map((key) => (
								<TableRow key={key}>
									<TableCell colSpan={4}>
										<div className="h-4 animate-pulse rounded bg-muted" />
									</TableCell>
								</TableRow>
							))}
						{entries?.length === 0 && (
							<TableRow>
								<TableCell
									colSpan={4}
									className="py-8 text-center text-muted-foreground text-sm"
								>
									No activity yet
								</TableCell>
							</TableRow>
						)}
						{entries?.map((entry, idx) => {
							const isNamespaceEvent =
								entry.eventType === EventType.NAMESPACE_LOCKED ||
								entry.eventType === EventType.NAMESPACE_UNLOCKED ||
								!entry.path;
							const key = `${entry.revision.toString()}-${entry.namespace}-${entry.path}-${idx}`;
							return (
								<TableRow key={key}>
									<TableCell>
										<EventTypeBadge type={entry.eventType} />
									</TableCell>
									<TableCell className="font-mono text-xs">
										{entry.namespace ? (
											<Link
												to={`/browse/${entry.namespace}`}
												className="hover:underline"
											>
												{entry.namespace}
											</Link>
										) : (
											<span className="text-muted-foreground">—</span>
										)}
									</TableCell>
									<TableCell className="max-w-[240px] truncate font-mono text-xs">
										{isNamespaceEvent ? (
											<span className="text-muted-foreground">—</span>
										) : (
											<Link
												to={`/config/${entry.namespace}${entry.path}`}
												className="hover:underline"
											>
												{entry.path}
											</Link>
										)}
									</TableCell>
									<TableCell className="text-right text-muted-foreground text-xs">
										{entry.timestamp
											? timeAgo(new Date(tsToMs(entry.timestamp)))
											: "—"}
									</TableCell>
								</TableRow>
							);
						})}
					</TableBody>
				</Table>
			</CardContent>
		</Card>
	);
}

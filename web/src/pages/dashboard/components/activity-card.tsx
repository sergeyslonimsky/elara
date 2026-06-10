import { Link } from "react-router";
import { EventTypeBadge } from "@/components/event-type-badge";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
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

const SKELETON_ROWS = 6;

interface ActivityCardProps {
	entries: ActivityEntry[] | undefined;
	isLoading: boolean;
	limit: number;
}

export function ActivityCard({
	entries,
	isLoading,
	limit,
}: Readonly<ActivityCardProps>) {
	return (
		<Card className="rounded-xl lg:col-span-2">
			<CardHeader className="pb-3">
				<CardTitle className="text-base font-semibold">
					Last {limit} Changes
				</CardTitle>
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
							Array.from({ length: SKELETON_ROWS }).map((_, idx) => (
								// biome-ignore lint/suspicious/noArrayIndexKey: stable length
								<TableRow key={idx}>
									<TableCell>
										<Skeleton className="h-5 w-20" />
									</TableCell>
									<TableCell>
										<Skeleton className="h-4 w-24" />
									</TableCell>
									<TableCell>
										<Skeleton className="h-4 w-48" />
									</TableCell>
									<TableCell className="text-right">
										<Skeleton className="ml-auto h-4 w-16" />
									</TableCell>
								</TableRow>
							))}
						{!isLoading && entries?.length === 0 && (
							<TableRow>
								<TableCell
									colSpan={4}
									className="py-12 text-center text-muted-foreground text-sm"
								>
									No activity yet
								</TableCell>
							</TableRow>
						)}
						{!isLoading &&
							entries?.map((entry, idx) => {
								const isNamespaceEvent =
									entry.eventType === EventType.NAMESPACE_LOCKED ||
									entry.eventType === EventType.NAMESPACE_UNLOCKED ||
									!entry.path;
								const key = `${entry.revision.toString()}-${entry.namespace}-${
									entry.path
								}-${idx}`;
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

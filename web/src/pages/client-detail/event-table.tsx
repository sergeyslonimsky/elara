import {
	Table,
	TableBody,
	TableCell,
	TableHead,
	TableHeader,
	TableRow,
} from "@/components/ui/table";
import type { ClientEvent } from "@/gen/elara/clients/v1/clients_pb";
import { formatRpcDuration } from "@/lib/duration";
import { tsToMs } from "@/lib/time";

/** Shared table for activity and error event lists. */
export function EventTable({
	events,
	highlightErrors = false,
}: Readonly<{
	events: ClientEvent[];
	highlightErrors?: boolean;
}>) {
	return (
		<div className="overflow-hidden rounded-md border">
			<Table>
				<TableHeader>
					<TableRow>
						<TableHead className="w-32">Time</TableHead>
						<TableHead>Method</TableHead>
						<TableHead>Key</TableHead>
						<TableHead className="w-24 text-right">Duration</TableHead>
						<TableHead>Error</TableHead>
					</TableRow>
				</TableHeader>
				<TableBody>
					{events.map((ev, idx) => {
						const t = ev.timestamp ? new Date(tsToMs(ev.timestamp)) : null;
						const rowClass =
							highlightErrors || ev.error ? "bg-destructive/5" : undefined;
						return (
							<TableRow
								// biome-ignore lint/suspicious/noArrayIndexKey: events have no stable id
								key={`${idx}-${tsToMs(ev.timestamp)}-${ev.method}`}
								className={rowClass}
							>
								<TableCell className="whitespace-nowrap text-muted-foreground text-xs">
									{t ? t.toLocaleTimeString() : "—"}
								</TableCell>
								<TableCell className="font-mono text-xs">{ev.method}</TableCell>
								<TableCell className="font-mono text-muted-foreground text-xs">
									{ev.key || "—"}
								</TableCell>
								<TableCell className="whitespace-nowrap text-right text-xs">
									{formatRpcDuration(ev.duration)}
								</TableCell>
								<TableCell
									className={
										highlightErrors
											? "font-medium text-destructive text-xs"
											: "text-destructive text-xs"
									}
								>
									{ev.error}
								</TableCell>
							</TableRow>
						);
					})}
				</TableBody>
			</Table>
		</div>
	);
}

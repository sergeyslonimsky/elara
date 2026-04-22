import { AlertTriangle, CheckCircle2 } from "lucide-react";
import {
	Empty,
	EmptyDescription,
	EmptyHeader,
	EmptyMedia,
	EmptyTitle,
} from "@/components/ui/empty";
import type { ClientEvent } from "@/gen/elara/clients/v1/clients_pb";
import { EventTable } from "./event-table";

export function ErrorsTab({
	events,
	totalErrors,
	isActive,
}: {
	events: ClientEvent[];
	totalErrors: number;
	isActive: boolean;
}) {
	const errorEvents = events.filter((ev) => ev.error !== "");

	if (errorEvents.length === 0) {
		if (totalErrors === 0) {
			return (
				<Empty>
					<EmptyHeader>
						<EmptyMedia variant="icon">
							<CheckCircle2 />
						</EmptyMedia>
						<EmptyTitle>No errors</EmptyTitle>
						<EmptyDescription>
							This client has not produced any failed RPCs.
						</EmptyDescription>
					</EmptyHeader>
				</Empty>
			);
		}

		if (!isActive) {
			// Historical clients: per-event detail isn't persisted, only the count is.
			return (
				<Empty>
					<EmptyHeader>
						<EmptyMedia variant="icon">
							<AlertTriangle />
						</EmptyMedia>
						<EmptyTitle>
							{totalErrors} error{totalErrors === 1 ? "" : "s"} during this
							session
						</EmptyTitle>
						<EmptyDescription>
							Per-error detail is not retained after a client disconnects — only
							the count is preserved in history.
						</EmptyDescription>
					</EmptyHeader>
				</Empty>
			);
		}

		// Active client — errors happened but rolled out of the in-memory buffer.
		return (
			<Empty>
				<EmptyHeader>
					<EmptyMedia variant="icon">
						<AlertTriangle />
					</EmptyMedia>
					<EmptyTitle>No recent errors</EmptyTitle>
					<EmptyDescription>
						{totalErrors} earlier error{totalErrors === 1 ? "" : "s"} occurred
						but they have rolled out of the in-memory event buffer.
					</EmptyDescription>
				</EmptyHeader>
			</Empty>
		);
	}

	const truncated = errorEvents.length < totalErrors;

	return (
		<div className="space-y-3">
			{truncated && (
				<p className="text-muted-foreground text-xs">
					Showing {errorEvents.length} of {totalErrors} total errors.{" "}
					{isActive
						? "Older errors have rolled out of the in-memory buffer."
						: "Per-error detail is not retained after disconnect; older entries are gone."}
				</p>
			)}
			<EventTable events={errorEvents} highlightErrors />
		</div>
	);
}

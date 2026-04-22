import { Eye } from "lucide-react";
import {
	Empty,
	EmptyDescription,
	EmptyHeader,
	EmptyMedia,
	EmptyTitle,
} from "@/components/ui/empty";
import type { ClientEvent } from "@/gen/elara/clients/v1/clients_pb";
import { EventTable } from "./event-table";

export function ActivityTab({
	events,
	isActive,
}: Readonly<{
	events: ClientEvent[];
	isActive: boolean;
}>) {
	if (events.length === 0) {
		return (
			<Empty>
				<EmptyHeader>
					<EmptyMedia variant="icon">
						<Eye />
					</EmptyMedia>
					<EmptyTitle>No activity yet</EmptyTitle>
					<EmptyDescription>
						{isActive
							? "Waiting for the next RPC..."
							: "This client did not produce any recorded activity."}
					</EmptyDescription>
				</EmptyHeader>
			</Empty>
		);
	}

	return <EventTable events={events} />;
}

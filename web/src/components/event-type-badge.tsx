import { Lock, LockOpen, Zap } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { EventType } from "@/gen/elara/config/v1/config_pb";

export function EventTypeBadge({ type }: { type: EventType }) {
	switch (type) {
		case EventType.CREATED:
			return (
				<Badge variant="success">
					<Zap className="mr-1 h-3 w-3" />
					Created
				</Badge>
			);
		case EventType.UPDATED:
			return <Badge variant="info">Updated</Badge>;
		case EventType.DELETED:
			return <Badge variant="destructive-soft">Deleted</Badge>;
		case EventType.LOCKED:
			return (
				<Badge variant="warning">
					<Lock className="mr-1 h-3 w-3" />
					Locked
				</Badge>
			);
		case EventType.UNLOCKED:
			return (
				<Badge variant="warning-soft">
					<LockOpen className="mr-1 h-3 w-3" />
					Unlocked
				</Badge>
			);
		case EventType.NAMESPACE_LOCKED:
			return (
				<Badge variant="warning">
					<Lock className="mr-1 h-3 w-3" />
					NS Locked
				</Badge>
			);
		case EventType.NAMESPACE_UNLOCKED:
			return (
				<Badge variant="warning-soft">
					<LockOpen className="mr-1 h-3 w-3" />
					NS Unlocked
				</Badge>
			);
		default:
			return <Badge variant="outline">Unknown</Badge>;
	}
}

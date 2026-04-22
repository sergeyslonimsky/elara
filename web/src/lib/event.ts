import { EventType } from "@/gen/elara/config/v1/config_pb";

export function eventTypeLabel(t: EventType): string {
	switch (t) {
		case EventType.CREATED:
			return "Created";
		case EventType.UPDATED:
			return "Updated";
		case EventType.DELETED:
			return "Deleted";
		case EventType.LOCKED:
			return "Locked";
		case EventType.UNLOCKED:
			return "Unlocked";
		case EventType.NAMESPACE_LOCKED:
			return "Namespace locked";
		case EventType.NAMESPACE_UNLOCKED:
			return "Namespace unlocked";
		default:
			return "Unknown";
	}
}

export function isLockEvent(t: EventType): boolean {
	return (
		t === EventType.LOCKED ||
		t === EventType.UNLOCKED ||
		t === EventType.NAMESPACE_LOCKED ||
		t === EventType.NAMESPACE_UNLOCKED
	);
}

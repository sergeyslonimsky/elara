import type { Duration } from "@bufbuild/protobuf/wkt";

/** formatUptime: 14m 32s, 2h 5m, 3d 4h, etc. Always two units. */
export function formatUptime(
	fromMs: number,
	toMs: number = Date.now(),
): string {
	const seconds = Math.max(0, Math.floor((toMs - fromMs) / 1000));

	const days = Math.floor(seconds / 86_400);
	const hours = Math.floor((seconds % 86_400) / 3600);
	const minutes = Math.floor((seconds % 3600) / 60);
	const secs = seconds % 60;

	if (days > 0) return `${days}d ${hours}h`;
	if (hours > 0) return `${hours}h ${minutes}m`;
	if (minutes > 0) return `${minutes}m ${secs}s`;
	return `${secs}s`;
}

/** formatRpcDuration: protobuf Duration → human ms/µs/ns string. */
export function formatRpcDuration(d: Duration | undefined): string {
	if (!d) return "—";
	const totalNanos = Number(d.seconds) * 1_000_000_000 + d.nanos;
	if (totalNanos < 1_000) return `${totalNanos}ns`;
	if (totalNanos < 1_000_000) return `${(totalNanos / 1_000).toFixed(1)}µs`;
	if (totalNanos < 1_000_000_000)
		return `${(totalNanos / 1_000_000).toFixed(1)}ms`;
	return `${(totalNanos / 1_000_000_000).toFixed(2)}s`;
}

/** isLive: true if last activity was within freshSeconds ago. */
export function isLive(
	lastActivityMs: number,
	freshSeconds: number = 5,
): boolean {
	return Date.now() - lastActivityMs < freshSeconds * 1000;
}

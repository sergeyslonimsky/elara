import type { Client } from "@/gen/elara/clients/v1/clients_pb";

/**
 * Sum all request counts across methods for a client.
 */
export function totalRequests(client: Client | undefined): number {
	if (!client) return 0;
	let total = 0;
	for (const v of Object.values(client.requestCounts ?? {})) {
		total += Number(v);
	}
	return total;
}

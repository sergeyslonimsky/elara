import { createConnectQueryKey } from "@connectrpc/connect-query";
import type { useQueryClient } from "@tanstack/react-query";
import {
	getConfig,
	getConfigHistory,
	listConfigs,
} from "@/gen/elara/config/v1/config_service-ConfigService_connectquery";
import { listNamespaces } from "@/gen/elara/namespace/v1/namespace_service-NamespaceService_connectquery";

export type QueryClient = ReturnType<typeof useQueryClient>;

const queryKeys = {
	namespaces: () =>
		createConnectQueryKey({ schema: listNamespaces, cardinality: undefined }),
	configs: () =>
		createConnectQueryKey({ schema: listConfigs, cardinality: undefined }),
	config: () =>
		createConnectQueryKey({ schema: getConfig, cardinality: undefined }),
	configHistory: () =>
		createConnectQueryKey({ schema: getConfigHistory, cardinality: undefined }),
} as const;

type QueryKey = keyof typeof queryKeys;

/**
 * Invalidate one server-side cached query family.
 *
 * Fire-and-forget: the returned Promise from `invalidateQueries` is intentionally
 * discarded — invalidation failures are non-fatal (worst case the cached data
 * stays until the next natural refetch) and we don't want every call site to
 * have to handle them.
 */
export function invalidate(client: QueryClient, key: QueryKey): void {
	client.invalidateQueries({ queryKey: queryKeys[key]() });
}

/**
 * Invalidate every config-related query in one call — used by mutations that
 * affect both the list and the detail views (lock/unlock, delete, restore).
 */
export function invalidateAllConfigData(client: QueryClient): void {
	invalidate(client, "configs");
	invalidate(client, "config");
	invalidate(client, "configHistory");
}

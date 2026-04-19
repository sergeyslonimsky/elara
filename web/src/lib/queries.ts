import { createConnectQueryKey } from "@connectrpc/connect-query";
import type { useQueryClient } from "@tanstack/react-query";
import { listConfigs } from "@/gen/elara/config/v1/config_service-ConfigService_connectquery";
import { listNamespaces } from "@/gen/elara/namespace/v1/namespace_service-NamespaceService_connectquery";

export const queryKeys = {
	namespaces: () =>
		createConnectQueryKey({
			schema: listNamespaces,
			cardinality: undefined,
		}),
	configs: () =>
		createConnectQueryKey({
			schema: listConfigs,
			cardinality: undefined,
		}),
} as const;

export type QueryClient = ReturnType<typeof useQueryClient>;

export function invalidateNamespaces(queryClient: QueryClient) {
	return queryClient.invalidateQueries({
		queryKey: queryKeys.namespaces(),
	});
}

export function invalidateConfigs(queryClient: QueryClient) {
	return queryClient.invalidateQueries({
		queryKey: queryKeys.configs(),
	});
}

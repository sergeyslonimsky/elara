import { useQuery } from "@connectrpc/connect-query";
import { listNamespaces } from "@/gen/elara/namespace/v1/namespace_service-NamespaceService_connectquery";

export function useNamespaces(query?: string) {
	return useQuery(listNamespaces, { query });
}

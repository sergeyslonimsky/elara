import { createConnectQueryKey } from "@connectrpc/connect-query";
import { useQueryClient } from "@tanstack/react-query";
import { useCallback } from "react";
import { ErrorCard } from "@/components/error-card";
import { getAuthInfo } from "@/gen/elara/auth/v1/auth_service-AuthService_connectquery";
import { Button } from "./ui/button";

interface FullScreenErrorProps {
	message: string;
}

export function FullScreenError({ message }: Readonly<FullScreenErrorProps>) {
	const queryClient = useQueryClient();

	const handleRetry = useCallback(() => {
		queryClient.invalidateQueries({
			queryKey: createConnectQueryKey({
				schema: getAuthInfo,
				cardinality: "finite",
			}),
		});
	}, [queryClient]);

	return (
		<div className="flex h-screen w-screen flex-col items-center justify-center gap-4">
			<ErrorCard message={message} />
			<Button variant="outline" onClick={handleRetry}>
				Retry
			</Button>
		</div>
	);
}

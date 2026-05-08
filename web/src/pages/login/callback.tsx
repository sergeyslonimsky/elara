import { createConnectQueryKey, useMutation } from "@connectrpc/connect-query";
import { useQueryClient } from "@tanstack/react-query";
import { Loader2 } from "lucide-react";
import { useEffect, useRef } from "react";
import { useNavigate, useSearchParams } from "react-router";
import { ErrorCard } from "@/components/error-card";
import {
	me,
	oIDCCallback,
} from "@/gen/elara/auth/v1/auth_service-AuthService_connectquery";

export function CallbackPage() {
	const [searchParams] = useSearchParams();
	const navigate = useNavigate();
	const queryClient = useQueryClient();
	const mutation = useMutation(oIDCCallback);
	const called = useRef(false);

	const code = searchParams.get("code");
	const state = searchParams.get("state");

	useEffect(() => {
		if (code && state && !called.current) {
			called.current = true;
			mutation.mutate(
				{ code, state },
				{
					onSuccess: async () => {
						await queryClient.invalidateQueries({
							queryKey: createConnectQueryKey({
								schema: me,
								cardinality: "finite",
							}),
						});
						navigate("/");
					},
				},
			);
		}
	}, [code, state, navigate, queryClient, mutation]);

	if (!code || !state) {
		return (
			<div className="flex min-h-screen items-center justify-center bg-muted/40 p-4">
				<ErrorCard message="Invalid Callback: Missing code or state from identity provider." />
			</div>
		);
	}

	if (mutation.isError) {
		return (
			<div className="flex min-h-screen items-center justify-center bg-muted/40 p-4">
				<ErrorCard
					message={mutation.error.message || "An unexpected error occurred"}
				/>
			</div>
		);
	}

	return (
		<div className="flex min-h-screen items-center justify-center bg-muted/40 p-4">
			<div className="flex flex-col items-center gap-4 text-center">
				<Loader2 className="h-8 w-8 animate-spin text-primary" />
				<div className="space-y-1">
					<h2 className="text-xl font-semibold">Authenticating</h2>
					<p className="text-sm text-muted-foreground">
						Please wait while we complete your sign-in.
					</p>
				</div>
			</div>
		</div>
	);
}

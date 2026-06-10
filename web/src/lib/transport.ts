import { Code, ConnectError, type Interceptor } from "@connectrpc/connect";
import { createConnectQueryKey } from "@connectrpc/connect-query";
import { createConnectTransport } from "@connectrpc/connect-web";
import type { QueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { me } from "@/gen/elara/profile/v1/profile_service-ProfileService_connectquery";

// RPCs where Unauthenticated is a legitimate business response, not "session expired".
// - basicLogin:   401 = wrong password, login form shows the error
// - oIDCCallback: 401 = bad code, callback page shows the error
// - getAuthInfo:  defensive; shouldn't return Unauthenticated
// - me:           Unauthenticated means "not logged in" — AuthGuard handles it via meError
const SKIP_AUTH_HANDLING = new Set<string>([
	"basicLogin",
	"oIDCCallback",
	"getAuthInfo",
	"me",
]);

export function createAuthInterceptor(queryClient: QueryClient): Interceptor {
	let sessionExpiredHandled = false;

	return (next) => async (req) => {
		try {
			return await next(req);
		} catch (err) {
			if (err instanceof ConnectError) {
				const skip = SKIP_AUTH_HANDLING.has(req.method.localName);
				if (
					!skip &&
					err.code === Code.Unauthenticated &&
					!sessionExpiredHandled
				) {
					sessionExpiredHandled = true;
					toast.error("Session expired", {
						description: "Please sign in again.",
					});
					queryClient.invalidateQueries({
						queryKey: createConnectQueryKey({
							schema: me,
							cardinality: "finite",
						}),
					});
					setTimeout(() => {
						sessionExpiredHandled = false;
					}, 5000);
				}
			}
			// Always rethrow — local error handlers on pages must still see the error.
			throw err;
		}
	};
}

export function createAuthAwareTransport(queryClient: QueryClient) {
	return createConnectTransport({
		baseUrl: window.location.origin,
		fetch: (input, init) =>
			globalThis.fetch(input, { ...init, credentials: "include" }),
		interceptors: [createAuthInterceptor(queryClient)],
	});
}

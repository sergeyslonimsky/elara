import { Code, ConnectError } from "@connectrpc/connect";
import { createConnectQueryKey } from "@connectrpc/connect-query";
import { QueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { me } from "@/gen/elara/profile/v1/profile_service-ProfileService_connectquery";
import { createAuthAwareTransport, createAuthInterceptor } from "./transport";

vi.mock("sonner", () => ({
	toast: {
		error: vi.fn(),
	},
}));

// Build a minimal DescMethod-like object to satisfy req.method.localName
function makeMethod(localName: string) {
	return { localName } as { localName: string };
}

describe("createAuthAwareTransport interceptor", () => {
	let queryClient: QueryClient;

	type MockReq = {
		method: { localName: string };
	};

	// Thin wrapper that calls the real interceptor with mock-friendly types.
	// The Interceptor type is (next: AnyFn) => AnyFn where AnyFn accepts
	// UnaryRequest | StreamRequest. We cast via unknown to keep tests readable.
	async function callInterceptor(
		interceptorFn: ReturnType<typeof createAuthInterceptor>,
		req: MockReq,
		next: (r: MockReq) => Promise<unknown>,
	): Promise<unknown> {
		const wrappedNext = (r: unknown) => next(r as MockReq);
		return interceptorFn(wrappedNext as Parameters<typeof interceptorFn>[0])(
			req as unknown as Parameters<ReturnType<typeof interceptorFn>>[0],
		);
	}

	beforeEach(() => {
		queryClient = new QueryClient({
			defaultOptions: { queries: { retry: false } },
		});
		vi.useFakeTimers();
	});

	afterEach(() => {
		vi.clearAllMocks();
		vi.useRealTimers();
		queryClient.clear();
	});

	it("invalidates the me query and shows a toast when Unauthenticated is thrown for a non-skip method", async () => {
		const invalidateQueriesSpy = vi.spyOn(queryClient, "invalidateQueries");
		const interceptor = createAuthInterceptor(queryClient);

		const req = { method: makeMethod("someAction") };
		const error = new ConnectError("session expired", Code.Unauthenticated);
		const next = vi.fn().mockRejectedValue(error);

		await expect(callInterceptor(interceptor, req, next)).rejects.toThrow(
			error,
		);

		expect(toast.error).toHaveBeenCalledWith("Session expired", {
			description: "Please sign in again.",
		});
		expect(invalidateQueriesSpy).toHaveBeenCalledWith({
			queryKey: createConnectQueryKey({ schema: me, cardinality: "finite" }),
		});
	});

	it("does NOT invalidate queries for skip-listed methods (basicLogin)", async () => {
		const invalidateQueriesSpy = vi.spyOn(queryClient, "invalidateQueries");
		const interceptor = createAuthInterceptor(queryClient);

		const req = { method: makeMethod("basicLogin") };
		const error = new ConnectError("unauthorized", Code.Unauthenticated);
		const next = vi.fn().mockRejectedValue(error);

		await expect(callInterceptor(interceptor, req, next)).rejects.toThrow(
			error,
		);

		expect(invalidateQueriesSpy).not.toHaveBeenCalled();
		expect(toast.error).not.toHaveBeenCalled();
	});

	it("does NOT invalidate queries for skip-listed methods (oIDCCallback)", async () => {
		const invalidateQueriesSpy = vi.spyOn(queryClient, "invalidateQueries");
		const interceptor = createAuthInterceptor(queryClient);

		const req = { method: makeMethod("oIDCCallback") };
		const error = new ConnectError("bad code", Code.Unauthenticated);
		const next = vi.fn().mockRejectedValue(error);

		await expect(callInterceptor(interceptor, req, next)).rejects.toThrow(
			error,
		);

		expect(invalidateQueriesSpy).not.toHaveBeenCalled();
	});

	it("does NOT invalidate queries for skip-listed methods (me)", async () => {
		const invalidateQueriesSpy = vi.spyOn(queryClient, "invalidateQueries");
		const interceptor = createAuthInterceptor(queryClient);

		const req = { method: makeMethod("me") };
		const error = new ConnectError("not logged in", Code.Unauthenticated);
		const next = vi.fn().mockRejectedValue(error);

		await expect(callInterceptor(interceptor, req, next)).rejects.toThrow(
			error,
		);

		expect(invalidateQueriesSpy).not.toHaveBeenCalled();
	});

	it("debounces: invalidateQueries is called only once for rapid parallel Unauthenticated errors", async () => {
		const invalidateQueriesSpy = vi.spyOn(queryClient, "invalidateQueries");
		const interceptor = createAuthInterceptor(queryClient);

		const req = { method: makeMethod("someAction") };
		const error = new ConnectError("expired", Code.Unauthenticated);
		const next = vi.fn().mockRejectedValue(error);

		await Promise.allSettled([
			callInterceptor(interceptor, req, next),
			callInterceptor(interceptor, req, next),
			callInterceptor(interceptor, req, next),
		]);

		expect(invalidateQueriesSpy).toHaveBeenCalledTimes(1);
		expect(toast.error).toHaveBeenCalledTimes(1);
	});

	it("resets debounce after 5 seconds so subsequent errors are handled again", async () => {
		const invalidateQueriesSpy = vi.spyOn(queryClient, "invalidateQueries");
		const interceptor = createAuthInterceptor(queryClient);

		const req = { method: makeMethod("someAction") };
		const error = new ConnectError("expired", Code.Unauthenticated);
		const next = vi.fn().mockRejectedValue(error);

		await expect(callInterceptor(interceptor, req, next)).rejects.toThrow();
		expect(invalidateQueriesSpy).toHaveBeenCalledTimes(1);

		// Advance timers past the 5 second debounce window
		vi.advanceTimersByTime(5001);

		await expect(callInterceptor(interceptor, req, next)).rejects.toThrow();
		expect(invalidateQueriesSpy).toHaveBeenCalledTimes(2);
	});

	it("passes through non-Unauthenticated errors without side effects", async () => {
		const invalidateQueriesSpy = vi.spyOn(queryClient, "invalidateQueries");
		const interceptor = createAuthInterceptor(queryClient);

		const req = { method: makeMethod("someAction") };
		const error = new ConnectError("not found", Code.NotFound);
		const next = vi.fn().mockRejectedValue(error);

		await expect(callInterceptor(interceptor, req, next)).rejects.toThrow(
			error,
		);

		expect(invalidateQueriesSpy).not.toHaveBeenCalled();
		expect(toast.error).not.toHaveBeenCalled();
	});

	it("does not swallow errors — always rethrows even when invalidating", async () => {
		const interceptor = createAuthInterceptor(queryClient);

		const req = { method: makeMethod("someAction") };
		const error = new ConnectError("expired", Code.Unauthenticated);
		const next = vi.fn().mockRejectedValue(error);

		const thrown = await callInterceptor(interceptor, req, next).catch(
			(e: unknown) => e,
		);
		expect(thrown).toBe(error);
	});

	it("createAuthAwareTransport returns a transport object without throwing", () => {
		// Smoke test: the factory should not throw during creation
		expect(() => createAuthAwareTransport(queryClient)).not.toThrow();
	});
});

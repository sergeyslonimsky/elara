import { createClient } from "@connectrpc/connect";
import { useTransport } from "@connectrpc/connect-query";
import { useEffect, useRef, useState } from "react";

import type { Client, ClientEvent } from "@/gen/elara/clients/v1/clients_pb";
import {
	ClientsService,
	WatchClientResponse_Kind,
} from "@/gen/elara/clients/v1/clients_service_pb";

export type StreamStatus =
	| "connecting"
	| "connected"
	| "reconnecting"
	| "disconnected"; // client gone — no reconnect

export interface WatchClientState {
	snapshot: Client | undefined;
	events: ClientEvent[]; // newest first
	status: StreamStatus;
}

interface Options {
	/** Cap of in-memory event history on the client side. Default 100. */
	eventCapacity?: number;
}

const RECONNECT_INITIAL_MS = 1_000;
const RECONNECT_MAX_MS = 30_000;

/**
 * useWatchClient subscribes to the WatchClient server stream for a single
 * client ID and exposes live snapshot + activity events.
 *
 * Behaviour:
 *   - Auto-reconnect with exponential backoff (1s → 2s → 4s → max 30s)
 *   - Pauses (closes stream) when the document is hidden; resumes on visibility
 *   - On DISCONNECTED frame: stops reconnecting (the client is gone for good)
 *   - Cleans up the underlying fetch on unmount via AbortController
 */
export function useWatchClient(
	id: string,
	opts: Options = {},
): WatchClientState {
	const transport = useTransport();
	const eventCapacity = opts.eventCapacity ?? 100;

	const [state, setState] = useState<WatchClientState>({
		snapshot: undefined,
		events: [],
		status: "connecting",
	});

	// Stable refs so the effect can read latest values without re-running.
	const stateRef = useRef(state);
	stateRef.current = state;

	useEffect(() => {
		if (!id) return;

		let cancelled = false;
		let backoffMs = RECONNECT_INITIAL_MS;
		let abortController = new AbortController();
		let visible = document.visibilityState !== "hidden";

		const onVisibilityChange = () => {
			const nowVisible = document.visibilityState !== "hidden";
			if (visible === nowVisible) return;
			visible = nowVisible;

			if (!nowVisible) {
				// Pause: kill current stream, the loop's catch will treat as disconnect
				abortController.abort();
				return;
			}

			// Resume: reset backoff and let the loop reconnect promptly
			backoffMs = RECONNECT_INITIAL_MS;
			abortController = new AbortController();
			void runOnce();
		};

		document.addEventListener("visibilitychange", onVisibilityChange);

		const sleep = (ms: number) =>
			new Promise<void>((resolve) => {
				const t = setTimeout(resolve, ms);
				abortController.signal.addEventListener("abort", () => {
					clearTimeout(t);
					resolve();
				});
			});

		async function runOnce(): Promise<"end" | "retry"> {
			if (cancelled || !visible) return "end";

			setState((prev) => ({
				...prev,
				status: prev.status === "connected" ? "reconnecting" : "connecting",
			}));

			const client = createClient(ClientsService, transport);

			try {
				const stream = client.watchClient(
					{ id },
					{ signal: abortController.signal },
				);

				for await (const resp of stream) {
					if (cancelled) return "end";

					backoffMs = RECONNECT_INITIAL_MS; // any successful frame resets backoff

					switch (resp.kind) {
						case WatchClientResponse_Kind.SNAPSHOT:
							setState((prev) => ({
								...prev,
								snapshot: resp.client,
								status: "connected",
							}));
							break;

						case WatchClientResponse_Kind.REQUEST_RECORDED:
							if (resp.event) {
								const ev = resp.event;
								setState((prev) => ({
									...prev,
									snapshot: resp.client ?? prev.snapshot,
									events: [ev, ...prev.events].slice(0, eventCapacity),
									status: "connected",
								}));
							}
							break;

						case WatchClientResponse_Kind.DISCONNECTED:
							// Authoritative terminal: the *etcd* client is gone for good.
							setState((prev) => ({
								...prev,
								snapshot: resp.client ?? prev.snapshot,
								status: "disconnected",
							}));
							return "end";
					}
				}

				// Stream ended without a DISCONNECTED frame. This happens when:
				//   - the HTTP server hits its WriteTimeout
				//   - a proxy/load-balancer rotates the connection
				//   - the underlying TCP is reset
				// All of these are transient — the etcd client itself may still be
				// connected. Retry until either the cleanup runs (component unmount)
				// or we observe a real DISCONNECTED frame after reconnecting.
				return "retry";
			} catch {
				// Either AbortController fired or the network blipped.
				if (cancelled || !visible) return "end";
				return "retry";
			}
		}

		async function loop() {
			while (!cancelled) {
				const result = await runOnce();
				if (result === "end") return;
				if (cancelled || !visible) return;

				await sleep(backoffMs);
				backoffMs = Math.min(backoffMs * 2, RECONNECT_MAX_MS);
				abortController = new AbortController();
			}
		}

		void loop();

		return () => {
			cancelled = true;
			abortController.abort();
			document.removeEventListener("visibilitychange", onVisibilityChange);
		};
	}, [transport, id, eventCapacity]);

	return state;
}

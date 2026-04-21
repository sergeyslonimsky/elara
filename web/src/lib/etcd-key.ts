/**
 * Decoded view of an etcd key under the Elara convention
 * "/{namespace}/{path}".
 */
export interface DecodedKey {
	namespace: string;
	/** Path within the namespace, always starting with "/". */
	path: string;
}

/**
 * splitEtcdKey decodes a Elara etcd key. Returns null if the key doesn't
 * match the expected shape (must start with "/" and have a non-empty
 * namespace segment).
 *
 * Examples:
 *   "/prod/foo.json"          → {namespace: "prod",    path: "/foo.json"}
 *   "/prod/services/api.yaml" → {namespace: "prod",    path: "/services/api.yaml"}
 *   "/prod"                   → {namespace: "prod",    path: "/"}
 *   "/prod/"                  → {namespace: "prod",    path: "/"}
 *   "bad"                     → null
 */
export function splitEtcdKey(key: string): DecodedKey | null {
	if (!key.startsWith("/")) return null;

	const rest = key.slice(1);
	const slashIdx = rest.indexOf("/");

	if (slashIdx < 0) {
		if (!rest) return null;
		return { namespace: rest, path: "/" };
	}

	const namespace = rest.slice(0, slashIdx);
	if (!namespace) return null;

	const path = rest.slice(slashIdx);
	return { namespace, path: path || "/" };
}

/**
 * isPrefixRange reports whether (start, end) form an etcd "prefix range" —
 * end is `start` with the last byte incremented (e.g. "/prod/" + "/prod0").
 */
export function isPrefixRange(start: string, end: string): boolean {
	if (!start || start.length !== end.length) return false;
	for (let i = 0; i < start.length - 1; i++) {
		if (start[i] !== end[i]) return false;
	}
	return (
		end.charCodeAt(end.length - 1) === start.charCodeAt(start.length - 1) + 1
	);
}

/**
 * Categorisation of a watch range for UI display.
 */
export type WatchTarget =
	| { kind: "key"; namespace: string; path: string }
	| { kind: "prefix"; namespace: string; path: string }
	| { kind: "all-in-namespace"; namespace: string }
	| { kind: "range"; namespace: string; startPath: string; endPath: string }
	| { kind: "scan-all"; namespace: string; path: string }
	| { kind: "raw"; startKey: string; endKey: string };

export function classifyWatch(startKey: string, endKey: string): WatchTarget {
	const start = splitEtcdKey(startKey);

	// Single-key watch (no range)
	if (!endKey) {
		if (!start) return { kind: "raw", startKey, endKey };
		return { kind: "key", namespace: start.namespace, path: start.path };
	}

	// Open-ended range "all keys >= start"
	if (endKey === "\u0000") {
		if (!start) return { kind: "raw", startKey, endKey };
		return { kind: "scan-all", namespace: start.namespace, path: start.path };
	}

	if (!start) return { kind: "raw", startKey, endKey };

	// Prefix range: end is start with last byte +1
	if (isPrefixRange(startKey, endKey)) {
		// Special case: "/{ns}/" + "/{ns}0" → all configs in namespace
		if (start.path === "/") {
			return { kind: "all-in-namespace", namespace: start.namespace };
		}
		return { kind: "prefix", namespace: start.namespace, path: start.path };
	}

	// Explicit range — only displayable cleanly when both ends are in the
	// same namespace.
	const end = splitEtcdKey(endKey);
	if (end && end.namespace === start.namespace) {
		return {
			kind: "range",
			namespace: start.namespace,
			startPath: start.path,
			endPath: end.path,
		};
	}

	return { kind: "raw", startKey, endKey };
}

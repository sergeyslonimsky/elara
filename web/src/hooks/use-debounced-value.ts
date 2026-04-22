import { useEffect, useState } from "react";

/**
 * Returns `value` lagged by `delayMs` — standard debounce.
 *
 * Useful for live-search inputs where firing an RPC on every keystroke is
 * wasteful. Example:
 *
 *   const [input, setInput] = useState("");
 *   const debounced = useDebouncedValue(input, 200);
 *   const { data } = useQuery(search, { query: debounced });
 */
export function useDebouncedValue<T>(value: T, delayMs: number): T {
	const [debounced, setDebounced] = useState(value);

	useEffect(() => {
		const t = setTimeout(() => setDebounced(value), delayMs);
		return () => clearTimeout(t);
	}, [value, delayMs]);

	return debounced;
}

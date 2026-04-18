import { useEffect, useRef, useState } from "react";

/**
 * useHistory accumulates a bounded series of values over time. Each render in
 * which `value` differs from the last appended sample produces one new entry.
 * Used for client-side sparklines off live snapshot streams.
 *
 * Note: since-mount only — page reload starts the series fresh.
 */
export function useHistory(value: number, capacity: number = 60): number[] {
	const lastRef = useRef<number | undefined>(undefined);
	const [series, setSeries] = useState<number[]>([]);

	useEffect(() => {
		if (lastRef.current === value) return;
		lastRef.current = value;
		setSeries((prev) => {
			const next = [...prev, value];
			if (next.length > capacity) next.splice(0, next.length - capacity);
			return next;
		});
	}, [value, capacity]);

	return series;
}

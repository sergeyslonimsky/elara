import { lazy, Suspense } from "react";

const SparklineImpl = lazy(() =>
	import("./sparkline-impl").then((m) => ({ default: m.Sparkline })),
);

/**
 * Sparkline defers loading of the recharts library until first render.
 * The fallback is a transparent placeholder that holds layout space.
 */
export function Sparkline({ data }: Readonly<{ data: number[] }>) {
	return (
		<Suspense fallback={<div className="h-full w-full" />}>
			<SparklineImpl data={data} />
		</Suspense>
	);
}

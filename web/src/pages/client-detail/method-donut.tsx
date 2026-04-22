import { lazy, Suspense } from "react";
import { Skeleton } from "@/components/ui/skeleton";
import type { Client } from "@/gen/elara/clients/v1/clients_pb";

const MethodDonutImpl = lazy(() =>
	import("./method-donut-impl").then((m) => ({ default: m.MethodDonut })),
);

/**
 * MethodDonut defers loading of recharts until the "Counters" tab is opened.
 */
export function MethodDonut({ client }: { client: Client }) {
	return (
		<Suspense fallback={<Skeleton className="h-full w-full rounded-lg" />}>
			<MethodDonutImpl client={client} />
		</Suspense>
	);
}

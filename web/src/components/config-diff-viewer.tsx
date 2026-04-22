import { lazy, Suspense } from "react";
import { Skeleton } from "@/components/ui/skeleton";

const ConfigDiffViewerImpl = lazy(() =>
	import("./config-diff-viewer-impl").then((m) => ({
		default: m.ConfigDiffViewer,
	})),
);

interface ConfigDiffViewerProps {
	original: string;
	modified: string;
	language?: string;
	height?: string;
	header?: React.ReactNode;
}

export function ConfigDiffViewer(props: ConfigDiffViewerProps) {
	const height = props.height ?? "400px";
	return (
		<Suspense
			fallback={<Skeleton className="w-full rounded-lg" style={{ height }} />}
		>
			<ConfigDiffViewerImpl {...props} />
		</Suspense>
	);
}

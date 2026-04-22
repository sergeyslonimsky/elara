import { lazy, Suspense } from "react";
import { Skeleton } from "@/components/ui/skeleton";

const ConfigEditorImpl = lazy(() =>
	import("./config-editor-impl").then((m) => ({ default: m.ConfigEditor })),
);

interface ConfigEditorProps {
	value: string;
	onChange?: (value: string) => void;
	language?: string;
	readOnly?: boolean;
	height?: string;
}

export function ConfigEditor(props: ConfigEditorProps) {
	const height = props.height ?? "400px";
	return (
		<Suspense
			fallback={<Skeleton className="w-full rounded-lg" style={{ height }} />}
		>
			<ConfigEditorImpl {...props} />
		</Suspense>
	);
}

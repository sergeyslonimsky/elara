import { useQuery } from "@connectrpc/connect-query";
import { ConfigDiffViewer } from "@/components/config-diff-viewer";
import { Skeleton } from "@/components/ui/skeleton";
import { getConfigDiff } from "@/gen/elara/config/v1/config_service-ConfigService_connectquery";

interface InlineDiffPanelProps {
	path: string;
	namespace: string;
	language: string;
	toRevision: bigint;
	fromRevision: bigint;
}

export function InlineDiffPanel({
	path,
	namespace,
	language,
	toRevision,
	fromRevision,
}: Readonly<InlineDiffPanelProps>) {
	const { data, isLoading, error } = useQuery(
		getConfigDiff,
		{ path, namespace, fromRevision, toRevision },
		{ staleTime: Number.POSITIVE_INFINITY },
	);

	if (isLoading) {
		return <Skeleton className="mt-2 h-48 w-full rounded-lg" />;
	}

	if (error) {
		return <p className="mt-2 text-destructive text-xs">{error.message}</p>;
	}

	if (!data) return null;

	return (
		<div className="mt-2">
			<ConfigDiffViewer
				original={data.fromContent}
				modified={data.toContent}
				language={language}
				height="240px"
			/>
		</div>
	);
}

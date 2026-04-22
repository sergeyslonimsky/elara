import { timestampDate } from "@bufbuild/protobuf/wkt";
import { Lock } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import type { Config } from "@/gen/elara/config/v1/config_pb";
import { formatLabel } from "@/lib/format";

interface ConfigHeaderProps {
	config: Config;
	namespace: string;
	path: string;
	configLocked: boolean;
	namespaceLocked: boolean;
}

export function ConfigHeader({
	config,
	namespace,
	path,
	configLocked,
	namespaceLocked,
}: ConfigHeaderProps) {
	const effectiveLocked = configLocked || namespaceLocked;

	return (
		<div className="flex flex-wrap items-center gap-3">
			<h1 className="font-semibold text-xl">{path.split("/").pop()}</h1>
			<Badge variant="secondary">{formatLabel(config.format)}</Badge>
			<Badge variant="outline">v{config.version}</Badge>
			{effectiveLocked && (
				<Badge
					variant="outline"
					className="gap-1 text-amber-600 border-amber-400"
					title={
						namespaceLocked
							? `Namespace "${namespace}" is locked`
							: "Config is locked"
					}
				>
					<Lock className="h-3 w-3" />
					{namespaceLocked ? "Namespace locked" : "Locked"}
				</Badge>
			)}
			<span className="text-muted-foreground text-xs">
				rev {config.revision}
			</span>
			{config.updatedAt && (
				<span className="text-muted-foreground text-xs">
					updated {timestampDate(config.updatedAt).toLocaleString()}
				</span>
			)}
			{config.createdAt && (
				<span className="text-muted-foreground text-xs">
					created {timestampDate(config.createdAt).toLocaleString()}
				</span>
			)}
		</div>
	);
}

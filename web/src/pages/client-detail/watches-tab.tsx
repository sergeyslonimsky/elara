import { Eye } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import {
	Empty,
	EmptyDescription,
	EmptyHeader,
	EmptyMedia,
	EmptyTitle,
} from "@/components/ui/empty";
import {
	Table,
	TableBody,
	TableCell,
	TableHead,
	TableHeader,
	TableRow,
} from "@/components/ui/table";
import type { Client } from "@/gen/elara/clients/v1/clients_pb";
import { classifyWatch, type WatchTarget } from "@/lib/etcd-key";
import { timeAgo, tsToMs } from "@/lib/time";

export function WatchesTab({
	client,
	isActive,
}: {
	client: Client;
	isActive: boolean;
}) {
	const watches = client.activeWatchList ?? [];

	if (!isActive) {
		return (
			<Empty>
				<EmptyHeader>
					<EmptyMedia variant="icon">
						<Eye />
					</EmptyMedia>
					<EmptyTitle>Watches not retained</EmptyTitle>
					<EmptyDescription>
						Per-watch detail is in-memory only and is dropped when a client
						disconnects. The final count was{" "}
						<span className="font-medium">{client.activeWatches}</span>.
					</EmptyDescription>
				</EmptyHeader>
			</Empty>
		);
	}

	if (watches.length === 0) {
		return (
			<Empty>
				<EmptyHeader>
					<EmptyMedia variant="icon">
						<Eye />
					</EmptyMedia>
					<EmptyTitle>No active watches</EmptyTitle>
					<EmptyDescription>
						This client has no open Watch streams.
					</EmptyDescription>
				</EmptyHeader>
			</Empty>
		);
	}

	const sorted = [...watches].sort(
		(a, b) => tsToMs(b.createdAt) - tsToMs(a.createdAt),
	);

	return (
		<div className="overflow-hidden rounded-md border">
			<Table>
				<TableHeader>
					<TableRow>
						<TableHead className="w-16">ID</TableHead>
						<TableHead className="w-24">Type</TableHead>
						<TableHead className="w-40">Namespace</TableHead>
						<TableHead>Path</TableHead>
						<TableHead className="w-28">Start rev</TableHead>
						<TableHead className="w-28">Created</TableHead>
						<TableHead className="w-28">Flags</TableHead>
					</TableRow>
				</TableHeader>
				<TableBody>
					{sorted.map((w) => {
						const target = classifyWatch(w.startKey, w.endKey);
						return (
							<TableRow key={String(w.watchId)}>
								<TableCell className="font-mono text-muted-foreground text-xs">
									{String(w.watchId)}
								</TableCell>
								<TableCell>
									<WatchTypeBadge target={target} />
								</TableCell>
								<TableCell>
									<WatchNamespaceCell target={target} />
								</TableCell>
								<TableCell>
									<WatchPathCell target={target} />
								</TableCell>
								<TableCell className="font-mono text-xs tabular-nums">
									{w.startRevision > 0n ? String(w.startRevision) : "now"}
								</TableCell>
								<TableCell className="text-muted-foreground text-xs">
									{w.createdAt ? timeAgo(new Date(tsToMs(w.createdAt))) : "—"}
								</TableCell>
								<TableCell>
									<div className="flex flex-wrap gap-1">
										{w.prevKv && (
											<Badge variant="outline" className="text-[10px]">
												prev_kv
											</Badge>
										)}
										{w.progressNotify && (
											<Badge variant="outline" className="text-[10px]">
												progress
											</Badge>
										)}
									</div>
								</TableCell>
							</TableRow>
						);
					})}
				</TableBody>
			</Table>
		</div>
	);
}

function WatchTypeBadge({ target }: { target: WatchTarget }) {
	switch (target.kind) {
		case "key":
			return (
				<Badge variant="secondary" className="text-[10px]">
					key
				</Badge>
			);
		case "prefix":
			return (
				<Badge variant="secondary" className="text-[10px]">
					prefix
				</Badge>
			);
		case "all-in-namespace":
			return (
				<Badge variant="secondary" className="text-[10px]">
					all in ns
				</Badge>
			);
		case "range":
			return (
				<Badge variant="secondary" className="text-[10px]">
					range
				</Badge>
			);
		case "scan-all":
			return (
				<Badge variant="secondary" className="text-[10px]">
					≥ key
				</Badge>
			);
		case "raw":
			return (
				<Badge variant="outline" className="text-[10px]">
					raw
				</Badge>
			);
	}
}

function WatchNamespaceCell({ target }: { target: WatchTarget }) {
	if (target.kind === "raw") {
		return <span className="text-muted-foreground italic">—</span>;
	}
	return (
		<Badge variant="outline" className="font-mono text-xs">
			{target.namespace}
		</Badge>
	);
}

function WatchPathCell({ target }: { target: WatchTarget }) {
	switch (target.kind) {
		case "key":
			return <span className="font-mono text-xs">{target.path}</span>;
		case "prefix":
			return (
				<span className="font-mono text-xs">
					{target.path}
					<span className="text-muted-foreground">*</span>
				</span>
			);
		case "all-in-namespace":
			return <span className="text-muted-foreground italic">all configs</span>;
		case "range":
			return (
				<span className="font-mono text-xs">
					{target.startPath} <span className="text-muted-foreground">→</span>{" "}
					{target.endPath}
				</span>
			);
		case "scan-all":
			return (
				<span className="font-mono text-xs">
					{target.path}{" "}
					<span className="text-muted-foreground">and beyond</span>
				</span>
			);
		case "raw":
			return (
				<span className="font-mono text-muted-foreground text-xs">
					{target.startKey}
					{target.endKey ? ` → ${target.endKey}` : ""}
				</span>
			);
	}
}

import { timestampDate } from "@bufbuild/protobuf/wkt";
import { GitCompare, Lock } from "lucide-react";
import { useState } from "react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { EventType, type HistoryEntry } from "@/gen/elara/config/v1/config_pb";
import { eventTypeLabel, isLockEvent } from "@/lib/event";
import { ComparePanel } from "./compare-panel";
import { InlineDiffPanel } from "./inline-diff-panel";

interface HistoryListProps {
	entries: HistoryEntry[];
	path: string;
	namespace: string;
	language: string;
	version: bigint;
}

export function HistoryList({
	entries,
	path,
	namespace,
	language,
	version,
}: HistoryListProps) {
	const [expandedRevision, setExpandedRevision] = useState<bigint | null>(null);
	const [compareMode, setCompareMode] = useState(false);

	return (
		<>
			<div className="mb-4 flex items-center justify-between">
				<span className="text-muted-foreground text-sm">
					{entries.length} revision{entries.length !== 1 ? "s" : ""}
				</span>
				{entries.length >= 2 && (
					<Button
						variant={compareMode ? "default" : "outline"}
						size="sm"
						onClick={() => {
							setCompareMode((v) => !v);
							setExpandedRevision(null);
						}}
					>
						<GitCompare className="mr-1 h-4 w-4" />
						{compareMode ? "Exit compare" : "Compare revisions"}
					</Button>
				)}
			</div>

			{entries.length === 0 ? (
				<p className="py-8 text-center text-muted-foreground">
					No history available
				</p>
			) : compareMode ? (
				<ComparePanel
					path={path}
					namespace={namespace}
					language={language}
					entries={entries}
					version={version}
				/>
			) : (
				<div className="space-y-3">
					{entries.map((entry, idx) => {
						if (isLockEvent(entry.eventType)) {
							return <LockEventRow key={lockRowKey(entry)} entry={entry} />;
						}

						const isExpanded = expandedRevision === entry.revision;
						const fromRevision = previousContentRevision(entries, idx);

						return (
							<ContentEventRow
								key={entry.revision}
								entry={entry}
								isExpanded={isExpanded}
								onToggle={() =>
									setExpandedRevision(isExpanded ? null : entry.revision)
								}
								path={path}
								namespace={namespace}
								language={language}
								fromRevision={fromRevision}
							/>
						);
					})}
				</div>
			)}
		</>
	);
}

function lockRowKey(entry: HistoryEntry): string {
	const ts = entry.timestamp ? Number(timestampDate(entry.timestamp)) : 0;
	return `lock-${entry.eventType}-${ts}`;
}

function previousContentRevision(entries: HistoryEntry[], idx: number): bigint {
	for (let j = idx + 1; j < entries.length; j++) {
		if (!isLockEvent(entries[j].eventType)) {
			return entries[j].revision;
		}
	}
	return 0n;
}

function LockEventRow({ entry }: { entry: HistoryEntry }) {
	return (
		<div className="flex items-start gap-3 rounded-lg border p-3">
			<Badge
				variant="outline"
				className="mt-0.5 shrink-0 gap-1 border-amber-400 text-amber-600"
			>
				<Lock className="h-3 w-3" />
				{eventTypeLabel(entry.eventType)}
			</Badge>
			<div className="min-w-0 flex-1">
				{entry.timestamp && (
					<span className="text-muted-foreground text-xs">
						{timestampDate(entry.timestamp).toLocaleString()}
					</span>
				)}
			</div>
		</div>
	);
}

interface ContentEventRowProps {
	entry: HistoryEntry;
	isExpanded: boolean;
	onToggle: () => void;
	path: string;
	namespace: string;
	language: string;
	fromRevision: bigint;
}

function ContentEventRow({
	entry,
	isExpanded,
	onToggle,
	path,
	namespace,
	language,
	fromRevision,
}: ContentEventRowProps) {
	return (
		<div>
			<button
				type="button"
				className="flex w-full cursor-pointer items-start gap-3 rounded-lg border p-3 text-left transition-colors hover:bg-muted/50"
				onClick={onToggle}
			>
				<Badge
					variant={
						entry.eventType === EventType.CREATED ? "default" : "secondary"
					}
					className="mt-0.5 shrink-0"
				>
					{eventTypeLabel(entry.eventType)}
				</Badge>
				<div className="min-w-0 flex-1">
					<div className="flex items-center gap-2 text-sm">
						<span className="font-mono text-muted-foreground">
							rev {entry.revision}
						</span>
						{entry.timestamp && (
							<span className="text-muted-foreground text-xs">
								{timestampDate(entry.timestamp).toLocaleString()}
							</span>
						)}
					</div>
				</div>
				<span className="text-muted-foreground text-xs">
					{isExpanded ? "▲ hide diff" : "▼ show diff"}
				</span>
			</button>

			{isExpanded && (
				<div className="mx-3 mb-3 rounded-b-lg border border-t-0 p-3">
					<InlineDiffPanel
						path={path}
						namespace={namespace}
						language={language}
						fromRevision={fromRevision}
						toRevision={entry.revision}
					/>
				</div>
			)}
		</div>
	);
}

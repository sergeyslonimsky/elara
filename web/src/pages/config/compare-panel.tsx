import { useMutation, useQuery } from "@connectrpc/connect-query";
import { useQueryClient } from "@tanstack/react-query";
import { useMemo, useState } from "react";
import { toast } from "sonner";
import { ConfigDiffViewer } from "@/components/config-diff-viewer";
import {
	AlertDialog,
	AlertDialogAction,
	AlertDialogCancel,
	AlertDialogContent,
	AlertDialogDescription,
	AlertDialogFooter,
	AlertDialogHeader,
	AlertDialogTitle,
	AlertDialogTrigger,
} from "@/components/ui/alert-dialog";
import { Button } from "@/components/ui/button";
import {
	Select,
	SelectContent,
	SelectItem,
	SelectTrigger,
	SelectValue,
} from "@/components/ui/select";
import { Skeleton } from "@/components/ui/skeleton";
import type { HistoryEntry } from "@/gen/elara/config/v1/config_pb";
import {
	getConfigDiff,
	updateConfig,
} from "@/gen/elara/config/v1/config_service-ConfigService_connectquery";
import { eventTypeLabel, isLockEvent } from "@/lib/event";
import { invalidateAllConfigData } from "@/lib/queries";
import { toastError } from "@/lib/toast";

interface ComparePanelProps {
	path: string;
	namespace: string;
	language: string;
	entries: HistoryEntry[];
	version: bigint;
}

export function ComparePanel({
	path,
	namespace,
	language,
	entries,
	version,
}: Readonly<ComparePanelProps>) {
	const queryClient = useQueryClient();
	const [compareFrom, setCompareFrom] = useState<string>("");
	const [compareTo, setCompareTo] = useState<string>("");

	const { entriesAsc, entriesDesc } = useMemo(() => {
		// Lock events carry no revision/content — exclude them from diff picker.
		const content = entries.filter((e) => !isLockEvent(e.eventType));
		const asc = [...content].sort((a, b) => Number(a.revision - b.revision));
		return { entriesAsc: asc, entriesDesc: [...asc].reverse() };
	}, [entries]);

	const fromRevision = compareFrom ? BigInt(compareFrom) : undefined;
	const toRevision = compareTo ? BigInt(compareTo) : undefined;

	const enabled =
		fromRevision !== undefined &&
		toRevision !== undefined &&
		fromRevision <= toRevision;

	const { data, isLoading, error } = useQuery(
		getConfigDiff,
		{
			path,
			namespace,
			fromRevision: fromRevision ?? 0n,
			toRevision: toRevision ?? 0n,
		},
		{ enabled },
	);

	const [restoreOpen, setRestoreOpen] = useState(false);

	const restoreMutation = useMutation(updateConfig, {
		onSuccess: () => {
			toast.success("Config restored");
			setRestoreOpen(false);
			invalidateAllConfigData(queryClient);
		},
		onError: toastError,
	});

	const fromInvalid =
		fromRevision !== undefined &&
		toRevision !== undefined &&
		fromRevision > toRevision;

	return (
		<div className="space-y-4">
			<div className="flex flex-wrap items-center gap-3">
				<div className="flex items-center gap-2">
					<span className="text-sm">From:</span>
					<Select
						value={compareFrom}
						onValueChange={(v) => setCompareFrom(v ?? "")}
					>
						<SelectTrigger className="w-48">
							<SelectValue placeholder="Select revision" />
						</SelectTrigger>
						<SelectContent>
							<SelectItem value="0">Empty (before first)</SelectItem>
							{entriesAsc.map((e) => (
								<SelectItem key={String(e.revision)} value={String(e.revision)}>
									rev {String(e.revision)} — {eventTypeLabel(e.eventType)}
								</SelectItem>
							))}
						</SelectContent>
					</Select>
				</div>
				<div className="flex items-center gap-2">
					<span className="text-sm">To:</span>
					<Select
						value={compareTo}
						onValueChange={(v) => setCompareTo(v ?? "")}
					>
						<SelectTrigger className="w-48">
							<SelectValue placeholder="Select revision" />
						</SelectTrigger>
						<SelectContent>
							{entriesDesc.map((e) => (
								<SelectItem key={String(e.revision)} value={String(e.revision)}>
									rev {String(e.revision)} — {eventTypeLabel(e.eventType)}
								</SelectItem>
							))}
						</SelectContent>
					</Select>
				</div>
				<Button
					variant="ghost"
					size="sm"
					onClick={() => {
						setCompareFrom("");
						setCompareTo("");
					}}
				>
					Reset
				</Button>
			</div>

			{fromInvalid && (
				<p className="text-sm text-yellow-600">
					"From" revision is newer than "To" revision.
				</p>
			)}

			{isLoading && <Skeleton className="h-64 w-full rounded-lg" />}
			{error && <p className="text-destructive text-sm">{error.message}</p>}

			{data && (
				<>
					<ConfigDiffViewer
						original={data.fromContent}
						modified={data.toContent}
						language={language}
						height="400px"
					/>
					{data.toContent && (
						<div className="flex justify-end">
							<AlertDialog open={restoreOpen} onOpenChange={setRestoreOpen}>
								<AlertDialogTrigger
									render={<Button variant="outline" size="sm" />}
								>
									Restore to "To" version
								</AlertDialogTrigger>
								<AlertDialogContent>
									<AlertDialogHeader>
										<AlertDialogTitle>Restore config?</AlertDialogTitle>
										<AlertDialogDescription>
											This will overwrite the current content with revision{" "}
											{compareTo}.
										</AlertDialogDescription>
									</AlertDialogHeader>
									<AlertDialogFooter>
										<AlertDialogCancel>Cancel</AlertDialogCancel>
										<AlertDialogAction
											disabled={restoreMutation.isPending}
											onClick={() =>
												restoreMutation.mutate({
													path,
													namespace,
													content: data.toContent,
													version,
												})
											}
										>
											{restoreMutation.isPending ? "Restoring..." : "Restore"}
										</AlertDialogAction>
									</AlertDialogFooter>
								</AlertDialogContent>
							</AlertDialog>
						</div>
					)}
				</>
			)}
		</div>
	);
}

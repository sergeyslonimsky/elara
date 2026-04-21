import { timestampDate } from "@bufbuild/protobuf/wkt";
import { useMutation, useQuery } from "@connectrpc/connect-query";
import { useQueryClient } from "@tanstack/react-query";
import {
	ArrowLeft,
	Clock,
	Copy,
	GitCompare,
	Lock,
	LockOpen,
	Pencil,
	Trash2,
} from "lucide-react";
import { useMemo, useState } from "react";
import { Link, useNavigate, useParams } from "react-router";
import { toast } from "sonner";
import { ConfigDiffViewer } from "@/components/config-diff-viewer";
import { ConfigEditor } from "@/components/config-editor";
import { ErrorCard } from "@/components/error-card";
import { NamespaceSelect } from "@/components/namespace-select";
import { PageHeader } from "@/components/page-header.tsx";
import { PathBreadcrumb } from "@/components/path-breadcrumb";
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
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import {
	Dialog,
	DialogContent,
	DialogFooter,
	DialogHeader,
	DialogTitle,
	DialogTrigger,
} from "@/components/ui/dialog";
import { Field, FieldLabel } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import {
	Select,
	SelectContent,
	SelectItem,
	SelectTrigger,
	SelectValue,
} from "@/components/ui/select";
import { Separator } from "@/components/ui/separator";
import { Skeleton } from "@/components/ui/skeleton";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { EventType, type HistoryEntry } from "@/gen/elara/config/v1/config_pb";
import {
	copyConfig,
	deleteConfig,
	getConfig,
	getConfigDiff,
	getConfigHistory,
	lockConfig,
	unlockConfig,
	updateConfig,
} from "@/gen/elara/config/v1/config_service-ConfigService_connectquery";
import { formatLabel, protoFormatToLanguage } from "@/lib/format";
import {
	invalidateConfig,
	invalidateConfigHistory,
	invalidateConfigs,
} from "@/lib/queries";

function eventTypeLabel(t: EventType): string {
	switch (t) {
		case EventType.CREATED:
			return "Created";
		case EventType.UPDATED:
			return "Updated";
		case EventType.DELETED:
			return "Deleted";
		default:
			return "Unknown";
	}
}

function CopyDialog({
	sourcePath,
	sourceNamespace,
}: {
	sourcePath: string;
	sourceNamespace: string;
}) {
	const [open, setOpen] = useState(false);
	const [destPath, setDestPath] = useState(sourcePath);
	const [destNamespace, setDestNamespace] = useState(sourceNamespace);
	const queryClient = useQueryClient();

	const mutation = useMutation(copyConfig, {
		onSuccess: () => {
			toast.success(`Config copied to ${destPath}`);
			setOpen(false);
			void invalidateConfigs(queryClient);
		},
		onError: (err) => toast.error(err.message),
	});

	return (
		<Dialog open={open} onOpenChange={setOpen}>
			<DialogTrigger render={<Button variant="outline" size="sm" />}>
				<Copy className="mr-1 h-4 w-4" />
				Copy
			</DialogTrigger>
			<DialogContent>
				<form
					onSubmit={(e) => {
						e.preventDefault();
						mutation.mutate({
							sourcePath,
							sourceNamespace,
							destinationPath: destPath,
							destinationNamespace: destNamespace,
						});
					}}
				>
					<DialogHeader>
						<DialogTitle>Copy Config</DialogTitle>
					</DialogHeader>
					<div className="grid gap-4 py-4">
						<Field>
							<FieldLabel>Destination Path</FieldLabel>
							<Input
								value={destPath}
								onChange={(e) => setDestPath(e.target.value)}
								required
							/>
						</Field>
						<Field>
							<FieldLabel>Destination Namespace</FieldLabel>
							<NamespaceSelect
								value={destNamespace}
								onChange={setDestNamespace}
							/>
						</Field>
					</div>
					<DialogFooter>
						<Button type="submit" disabled={mutation.isPending}>
							{mutation.isPending ? "Copying..." : "Copy"}
						</Button>
					</DialogFooter>
				</form>
			</DialogContent>
		</Dialog>
	);
}

function InlineDiffPanel({
	path,
	namespace,
	language,
	toRevision,
	fromRevision,
}: {
	path: string;
	namespace: string;
	language: string;
	toRevision: bigint;
	fromRevision: bigint;
}) {
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

function ComparePanel({
	path,
	namespace,
	language,
	entries,
	version,
}: {
	path: string;
	namespace: string;
	language: string;
	entries: HistoryEntry[];
	version: bigint;
}) {
	const queryClient = useQueryClient();
	const [compareFrom, setCompareFrom] = useState<string>("");
	const [compareTo, setCompareTo] = useState<string>("");

	const { entriesAsc, entriesDesc } = useMemo(() => {
		const asc = [...entries].sort((a, b) => Number(a.revision - b.revision));
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
			void invalidateConfig(queryClient);
			void invalidateConfigHistory(queryClient);
		},
		onError: (err) => toast.error(err.message),
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

export function ConfigPage() {
	const { namespace = "default", "*": splat = "" } = useParams();
	const path = `/${splat}`;
	const navigate = useNavigate();
	const queryClient = useQueryClient();

	const { data, isLoading, error } = useQuery(getConfig, {
		path,
		namespace,
	});

	const { data: historyData } = useQuery(getConfigHistory, {
		path,
		namespace,
		limit: 20,
	});

	const parentPath = path.split("/").slice(0, -1).join("/") || "/";

	const deleteMutation = useMutation(deleteConfig, {
		onSuccess: () => {
			toast.success("Config deleted");
			void invalidateConfigs(queryClient);
			void invalidateConfig(queryClient);
			navigate(`/browse/${namespace}${parentPath}`);
		},
		onError: (err) => toast.error(err.message),
	});

	const lockMutation = useMutation(lockConfig, {
		onSuccess: () => {
			toast.success("Config locked");
			void invalidateConfig(queryClient);
			void invalidateConfigs(queryClient);
		},
		onError: (err) => toast.error(err.message),
	});

	const unlockMutation = useMutation(unlockConfig, {
		onSuccess: () => {
			toast.success("Config unlocked");
			void invalidateConfig(queryClient);
			void invalidateConfigs(queryClient);
		},
		onError: (err) => toast.error(err.message),
	});

	const [expandedRevision, setExpandedRevision] = useState<bigint | null>(null);
	const [compareMode, setCompareMode] = useState(false);

	const entries = historyData?.entries ?? [];
	const language = data?.config
		? protoFormatToLanguage(data.config.format)
		: "plaintext";

	return (
		<>
			<PageHeader title="Config Details" />
			<div className="flex flex-1 flex-col gap-4 p-4 pt-0">
				<div className="mt-4 flex items-center gap-4">
					<Button
						variant="ghost"
						size="sm"
						render={<Link to={`/browse/${namespace}${parentPath}`} />}
					>
						<ArrowLeft className="mr-1 h-4 w-4" />
						Back
					</Button>
					<PathBreadcrumb namespace={namespace} path={path} />
				</div>

				{isLoading && (
					<div className="space-y-4">
						<Skeleton className="h-8 w-48" />
						<Skeleton className="h-64 w-full rounded-xl" />
					</div>
				)}

				{error && <ErrorCard message={error.message} />}

				{data?.config && (
					<div className="space-y-4">
						<div className="flex flex-wrap items-center gap-3">
							<h1 className="font-semibold text-xl">{path.split("/").pop()}</h1>
							<Badge variant="secondary">
								{formatLabel(data.config.format)}
							</Badge>
							<Badge variant="outline">v{data.config.version}</Badge>
							{data.config.locked && (
								<Badge
									variant="outline"
									className="gap-1 text-amber-600 border-amber-400"
								>
									<Lock className="h-3 w-3" />
									Locked
								</Badge>
							)}
							<span className="text-muted-foreground text-xs">
								rev {data.config.revision}
							</span>
							{data.config.updatedAt && (
								<span className="text-muted-foreground text-xs">
									updated{" "}
									{timestampDate(data.config.updatedAt).toLocaleString()}
								</span>
							)}
							{data.config.createdAt && (
								<span className="text-muted-foreground text-xs">
									created{" "}
									{timestampDate(data.config.createdAt).toLocaleString()}
								</span>
							)}
						</div>

						<div className="flex flex-wrap gap-2">
							{data.config.locked ? (
								<Button variant="outline" size="sm" disabled>
									<Pencil className="mr-1 h-4 w-4" />
									Edit
								</Button>
							) : (
								<Button
									variant="outline"
									size="sm"
									render={<Link to={`/config/edit/${namespace}${path}`} />}
								>
									<Pencil className="mr-1 h-4 w-4" />
									Edit
								</Button>
							)}
							<CopyDialog sourcePath={path} sourceNamespace={namespace} />
							<AlertDialog>
								<AlertDialogTrigger
									render={<Button variant="outline" size="sm" />}
								>
									{data.config.locked ? (
										<>
											<LockOpen className="mr-1 h-4 w-4" />
											Unlock
										</>
									) : (
										<>
											<Lock className="mr-1 h-4 w-4" />
											Lock
										</>
									)}
								</AlertDialogTrigger>
								<AlertDialogContent>
									<AlertDialogHeader>
										<AlertDialogTitle>
											{data.config.locked ? "Unlock config?" : "Lock config?"}
										</AlertDialogTitle>
										<AlertDialogDescription>
											{data.config.locked
												? "Unlocking will allow this config to be updated or deleted."
												: "Locking will prevent this config from being updated or deleted until unlocked."}
										</AlertDialogDescription>
									</AlertDialogHeader>
									<AlertDialogFooter>
										<AlertDialogCancel>Cancel</AlertDialogCancel>
										<AlertDialogAction
											disabled={
												lockMutation.isPending || unlockMutation.isPending
											}
											onClick={() =>
												data.config?.locked
													? unlockMutation.mutate({ path, namespace })
													: lockMutation.mutate({ path, namespace })
											}
										>
											{data.config.locked
												? unlockMutation.isPending
													? "Unlocking..."
													: "Unlock"
												: lockMutation.isPending
													? "Locking..."
													: "Lock"}
										</AlertDialogAction>
									</AlertDialogFooter>
								</AlertDialogContent>
							</AlertDialog>
							<AlertDialog>
								<AlertDialogTrigger
									render={
										<Button
											variant="destructive"
											size="sm"
											disabled={data.config.locked ? true : undefined}
										/>
									}
								>
									<Trash2 className="mr-1 h-4 w-4" />
									Delete
								</AlertDialogTrigger>
								<AlertDialogContent>
									<AlertDialogHeader>
										<AlertDialogTitle>Delete config?</AlertDialogTitle>
										<AlertDialogDescription>
											This will permanently delete <strong>{path}</strong> from
											namespace <strong>{namespace}</strong>.
										</AlertDialogDescription>
									</AlertDialogHeader>
									<AlertDialogFooter>
										<AlertDialogCancel>Cancel</AlertDialogCancel>
										<AlertDialogAction
											className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
											disabled={deleteMutation.isPending}
											onClick={() => deleteMutation.mutate({ path, namespace })}
										>
											{deleteMutation.isPending ? "Deleting..." : "Delete"}
										</AlertDialogAction>
									</AlertDialogFooter>
								</AlertDialogContent>
							</AlertDialog>
						</div>

						<Separator />

						<Tabs defaultValue="content">
							<TabsList>
								<TabsTrigger value="content">Content</TabsTrigger>
								<TabsTrigger value="history">
									<Clock className="mr-1 h-3.5 w-3.5" />
									History
								</TabsTrigger>
							</TabsList>

							<TabsContent value="content" className="space-y-4">
								<Card className="rounded-xl">
									<CardContent className="pt-6">
										<ConfigEditor
											value={data.config.content}
											onChange={() => {}}
											language={language}
											readOnly
										/>
									</CardContent>
								</Card>

								{data.config.metadata &&
									Object.keys(data.config.metadata).length > 0 && (
										<Card className="rounded-xl">
											<CardHeader>
												<CardTitle className="text-sm">Metadata</CardTitle>
											</CardHeader>
											<CardContent>
												<div className="flex flex-wrap gap-2">
													{Object.entries(data.config.metadata).map(
														([key, value]) => (
															<Badge key={key} variant="outline">
																{key}: {value}
															</Badge>
														),
													)}
												</div>
											</CardContent>
										</Card>
									)}
							</TabsContent>

							<TabsContent value="history">
								<Card className="rounded-xl">
									<CardContent className="pt-6">
										<div className="mb-4 flex items-center justify-between">
											<span className="text-muted-foreground text-sm">
												{entries.length} revision
												{entries.length !== 1 ? "s" : ""}
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
												version={data.config.version}
											/>
										) : (
											<div className="space-y-3">
												{entries.map((entry, idx) => {
													const isExpanded =
														expandedRevision === entry.revision;
													const prevEntry = entries[idx + 1];
													const fromRevision = prevEntry
														? prevEntry.revision
														: 0n;

													return (
														<div key={entry.revision}>
															<button
																type="button"
																className="flex w-full cursor-pointer items-start gap-3 rounded-lg border p-3 text-left transition-colors hover:bg-muted/50"
																onClick={() =>
																	setExpandedRevision(
																		isExpanded ? null : entry.revision,
																	)
																}
															>
																<Badge
																	variant={
																		entry.eventType === EventType.CREATED
																			? "default"
																			: "secondary"
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
																				{timestampDate(
																					entry.timestamp,
																				).toLocaleString()}
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
												})}
											</div>
										)}
									</CardContent>
								</Card>
							</TabsContent>
						</Tabs>
					</div>
				)}
			</div>
		</>
	);
}

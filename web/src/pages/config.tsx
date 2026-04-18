import { timestampDate } from "@bufbuild/protobuf/wkt";
import {
	createConnectQueryKey,
	useMutation,
	useQuery,
} from "@connectrpc/connect-query";
import { useQueryClient } from "@tanstack/react-query";
import { ArrowLeft, Clock, Copy, Pencil, Trash2 } from "lucide-react";
import { useState } from "react";
import { Link, useNavigate, useParams } from "react-router";
import { toast } from "sonner";
import { AppHeader } from "@/components/app-header";
import { ConfigEditor } from "@/components/config-editor";
import { ErrorCard } from "@/components/error-card";
import { NamespaceSelect } from "@/components/namespace-select";
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
import { Separator } from "@/components/ui/separator";
import { Skeleton } from "@/components/ui/skeleton";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { EventType } from "@/gen/elara/config/v1/config_pb";
import {
	copyConfig,
	deleteConfig,
	getConfig,
	getConfigHistory,
	listConfigs,
} from "@/gen/elara/config/v1/config_service-ConfigService_connectquery";
import { formatLabel, protoFormatToLanguage } from "@/lib/format";

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
			void queryClient.invalidateQueries({
				queryKey: createConnectQueryKey({
					schema: listConfigs,
					cardinality: undefined,
				}),
			});
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
			void queryClient.invalidateQueries({
				queryKey: createConnectQueryKey({
					schema: listConfigs,
					cardinality: undefined,
				}),
			});
			void queryClient.invalidateQueries({
				queryKey: createConnectQueryKey({
					schema: getConfig,
					cardinality: undefined,
				}),
			});
			navigate(`/browse/${namespace}${parentPath}`);
		},
		onError: (err) => toast.error(err.message),
	});

	return (
		<>
			<AppHeader />
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
							<Button
								variant="outline"
								size="sm"
								render={<Link to={`/config/edit/${namespace}${path}`} />}
							>
								<Pencil className="mr-1 h-4 w-4" />
								Edit
							</Button>
							<CopyDialog sourcePath={path} sourceNamespace={namespace} />
							<AlertDialog>
								<AlertDialogTrigger
									render={<Button variant="destructive" size="sm" />}
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
											language={protoFormatToLanguage(data.config.format)}
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
										{historyData?.entries && historyData.entries.length > 0 ? (
											<div className="space-y-3">
												{historyData.entries.map((entry) => (
													<div
														key={entry.revision}
														className="flex items-start gap-3 rounded-lg border p-3"
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
															<pre className="mt-2 max-h-32 overflow-auto rounded bg-muted p-2 font-mono text-xs">
																{entry.content}
															</pre>
														</div>
													</div>
												))}
											</div>
										) : (
											<p className="py-8 text-center text-muted-foreground">
												No history available
											</p>
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

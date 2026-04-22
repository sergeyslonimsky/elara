import { useQuery } from "@connectrpc/connect-query";
import { ArrowLeft, Clock } from "lucide-react";
import { Link, useParams } from "react-router";
import { ConfigEditor } from "@/components/config-editor";
import { ErrorCard } from "@/components/error-card";
import { PageShell } from "@/components/page-shell";
import { PathBreadcrumb } from "@/components/path-breadcrumb";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Separator } from "@/components/ui/separator";
import { Skeleton } from "@/components/ui/skeleton";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import {
	getConfig,
	getConfigHistory,
} from "@/gen/elara/config/v1/config_service-ConfigService_connectquery";
import { useBackLink } from "@/hooks/use-back-link";
import { protoFormatToLanguage } from "@/lib/format";
import { ConfigActions } from "./config-actions";
import { ConfigHeader } from "./config-header";
import { HistoryList } from "./history-list";

export function ConfigPage() {
	const { namespace: namespaceParam, "*": splat = "" } = useParams();
	const namespace = namespaceParam ?? "";
	const path = `/${splat}`;

	const { data, isLoading, error } = useQuery(getConfig, {
		path,
		namespace,
	});

	const { data: historyData } = useQuery(getConfigHistory, {
		path,
		namespace,
		limit: 20,
	});

	const backLink = useBackLink(namespace, path);
	const entries = historyData?.entries ?? [];
	const language = data?.config
		? protoFormatToLanguage(data.config.format)
		: "plaintext";

	const configLocked = data?.config?.locked ?? false;
	const namespaceLocked = data?.config?.namespaceLocked ?? false;

	return (
		<PageShell title="Config Details">
			<div className="flex min-h-7 items-center">
				<PathBreadcrumb namespace={namespace} path={path} />
			</div>
			<div>
				<Button variant="ghost" size="sm" render={<Link to={backLink} />}>
					<ArrowLeft className="mr-1 h-4 w-4" />
					Back
				</Button>
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
					<ConfigHeader
						config={data.config}
						namespace={namespace}
						path={path}
						configLocked={configLocked}
						namespaceLocked={namespaceLocked}
					/>

					<ConfigActions
						path={path}
						namespace={namespace}
						configLocked={configLocked}
						namespaceLocked={namespaceLocked}
					/>

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
									<HistoryList
										entries={entries}
										path={path}
										namespace={namespace}
										language={language}
										version={data.config.version}
									/>
								</CardContent>
							</Card>
						</TabsContent>
					</Tabs>
				</div>
			)}
		</PageShell>
	);
}

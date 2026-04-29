import { useQuery } from "@connectrpc/connect-query";
import { ArrowLeft, CheckCircle2, Clock, XCircle } from "lucide-react";
import { Link, useParams } from "react-router";
import { ErrorCard } from "@/components/error-card";
import { PageShell } from "@/components/page-shell";
import { buttonVariants } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
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
import {
	getDeliveryHistory,
	getWebhook,
} from "@/gen/elara/webhook/v1/webhook_service-WebhookService_connectquery";
import { timeAgo, tsToMs } from "@/lib/time";
import { cn } from "@/lib/utils";

export function WebhookHistoryPage() {
	const { id = "" } = useParams();

	const webhookQ = useQuery(getWebhook, { id }, { enabled: !!id });
	const historyQ = useQuery(
		getDeliveryHistory,
		{ webhookId: id },
		{ enabled: !!id },
	);

	const webhook = webhookQ.data?.webhook;
	const attempts = historyQ.data?.attempts ?? [];
	const isLoading = webhookQ.isLoading || historyQ.isLoading;

	return (
		<PageShell title="Delivery History">
			<div>
				<Link
					to="/webhooks"
					className={cn(buttonVariants({ variant: "ghost", size: "sm" }))}
				>
					<ArrowLeft className="mr-1 h-4 w-4" />
					Back to webhooks
				</Link>
			</div>

			{webhook && (
				<p className="font-mono text-sm text-muted-foreground break-all">
					{webhook.url}
				</p>
			)}

			{(webhookQ.error ?? historyQ.error) && (
				<ErrorCard
					message={
						(webhookQ.error ?? historyQ.error)?.message ?? "Failed to load"
					}
				/>
			)}

			{!historyQ.error && (
				<Card className="rounded-xl">
					<CardContent className="pt-4">
						{!isLoading && attempts.length === 0 ? (
							<Empty>
								<EmptyHeader>
									<EmptyMedia variant="icon">
										<Clock />
									</EmptyMedia>
									<EmptyTitle>No delivery attempts yet</EmptyTitle>
									<EmptyDescription>
										Attempts will appear here once the webhook fires.
									</EmptyDescription>
								</EmptyHeader>
							</Empty>
						) : (
							<Table>
								<TableHeader>
									<TableRow>
										<TableHead className="w-12">#</TableHead>
										<TableHead className="w-16">Status</TableHead>
										<TableHead className="w-20">HTTP</TableHead>
										<TableHead className="w-28">Latency</TableHead>
										<TableHead>Error</TableHead>
										<TableHead className="w-28">Time</TableHead>
									</TableRow>
								</TableHeader>
								<TableBody>
									{attempts.map((a) => (
										<TableRow key={a.attemptNumber}>
											<TableCell className="text-muted-foreground">
												{a.attemptNumber}
											</TableCell>
											<TableCell>
												{a.success ? (
													<CheckCircle2 className="h-4 w-4 text-emerald-600" />
												) : (
													<XCircle className="h-4 w-4 text-destructive" />
												)}
											</TableCell>
											<TableCell>
												{a.statusCode > 0 ? (
													<span
														className={
															a.success
																? "text-emerald-600"
																: "text-destructive"
														}
													>
														{a.statusCode}
													</span>
												) : (
													<span className="text-muted-foreground">—</span>
												)}
											</TableCell>
											<TableCell>
												<span className="flex items-center gap-1 text-xs text-muted-foreground">
													<Clock className="h-3 w-3" />
													{String(a.latencyMs)}ms
												</span>
											</TableCell>
											<TableCell className="max-w-xs truncate text-xs text-muted-foreground">
												{a.error || "—"}
											</TableCell>
											<TableCell className="text-xs text-muted-foreground">
												{a.timestamp
													? timeAgo(new Date(tsToMs(a.timestamp)))
													: "—"}
											</TableCell>
										</TableRow>
									))}
								</TableBody>
							</Table>
						)}
					</CardContent>
				</Card>
			)}
		</PageShell>
	);
}

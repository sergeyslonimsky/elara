import { useMutation, useQuery } from "@connectrpc/connect-query";
import { useQueryClient } from "@tanstack/react-query";
import { Plus, Webhook } from "lucide-react";
import { useCallback, useMemo, useState } from "react";
import { DataTable } from "@/components/data-table";
import { ErrorCard } from "@/components/error-card";
import { PageShell } from "@/components/page-shell";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import {
	Empty,
	EmptyDescription,
	EmptyHeader,
	EmptyMedia,
	EmptyTitle,
} from "@/components/ui/empty";
import type { Webhook as WebhookType } from "@/gen/elara/webhook/v1/webhook_pb";
import {
	listWebhooks,
	updateWebhook,
} from "@/gen/elara/webhook/v1/webhook_service-WebhookService_connectquery";
import { invalidate } from "@/lib/queries";
import { toastError } from "@/lib/toast";
import { makeColumns } from "./columns";
import { WebhookSheet } from "./webhook-sheet";

export function WebhooksPage() {
	const queryClient = useQueryClient();
	const [sheetOpen, setSheetOpen] = useState(false);
	const [editingWebhook, setEditingWebhook] = useState<
		WebhookType | undefined
	>();

	const { data, isLoading, error, refetch, isFetching } = useQuery(
		listWebhooks,
		{},
	);

	const toggleMutation = useMutation(updateWebhook, {
		onSuccess: () => invalidate(queryClient, "webhooks"),
		onError: toastError,
	});

	function openCreate() {
		setEditingWebhook(undefined);
		setSheetOpen(true);
	}

	const openEdit = useCallback((webhook: WebhookType) => {
		setEditingWebhook(webhook);
		setSheetOpen(true);
	}, []);

	const handleToggleEnabled = useCallback(
		(webhook: WebhookType, enabled: boolean) => {
			toggleMutation.mutate({
				id: webhook.id,
				url: webhook.url,
				namespaceFilter: webhook.namespaceFilter,
				pathPrefix: webhook.pathPrefix,
				events: [...webhook.events],
				secret: "",
				enabled,
			});
		},
		[toggleMutation],
	);

	const columns = useMemo(
		() =>
			makeColumns({ onEdit: openEdit, onToggleEnabled: handleToggleEnabled }),
		[openEdit, handleToggleEnabled],
	);

	const webhooks = data?.webhooks ?? [];

	return (
		<>
			<PageShell
				title="Webhooks"
				onRefresh={() => refetch()}
				isRefreshing={isFetching}
				headerSlot={
					<Button size="sm" onClick={openCreate}>
						<Plus className="mr-1 h-4 w-4" />
						Add Webhook
					</Button>
				}
			>
				{error && <ErrorCard message={error.message} />}

				{!error && (
					<Card className="rounded-xl">
						<CardContent className="pt-4">
							{webhooks.length === 0 && !isLoading ? (
								<Empty>
									<EmptyHeader>
										<EmptyMedia variant="icon">
											<Webhook />
										</EmptyMedia>
										<EmptyTitle>No webhooks</EmptyTitle>
										<EmptyDescription>
											Add a webhook to receive push notifications when configs
											change.
										</EmptyDescription>
									</EmptyHeader>
								</Empty>
							) : (
								<DataTable
									columns={columns}
									data={webhooks}
									hideBorder
									nameColumnWidth="w-[40%]"
								/>
							)}
						</CardContent>
					</Card>
				)}
			</PageShell>

			<WebhookSheet
				open={sheetOpen}
				onOpenChange={setSheetOpen}
				webhook={editingWebhook}
			/>
		</>
	);
}

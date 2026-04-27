import type { ColumnDef } from "@tanstack/react-table";
import { History, Pencil } from "lucide-react";
import { Link } from "react-router";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import type { Webhook } from "@/gen/elara/webhook/v1/webhook_pb";
import { WebhookEvent } from "@/gen/elara/webhook/v1/webhook_pb";
import { DeleteDialog } from "./delete-dialog";

const EVENT_LABELS: Record<WebhookEvent, string> = {
	[WebhookEvent.UNSPECIFIED]: "",
	[WebhookEvent.CREATED]: "created",
	[WebhookEvent.UPDATED]: "updated",
	[WebhookEvent.DELETED]: "deleted",
};

interface ColumnOptions {
	onEdit: (webhook: Webhook) => void;
	onToggleEnabled: (webhook: Webhook, enabled: boolean) => void;
}

export function makeColumns({
	onEdit,
	onToggleEnabled,
}: ColumnOptions): ColumnDef<Webhook>[] {
	return [
		{
			id: "url",
			header: "URL",
			cell: ({ row }) => (
				<span
					className="font-mono text-xs truncate max-w-[260px] block"
					title={row.original.url}
				>
					{row.original.url}
				</span>
			),
		},
		{
			id: "events",
			header: "Events",
			cell: ({ row }) => (
				<div className="flex flex-wrap gap-1">
					{row.original.events
						.filter((e) => e !== WebhookEvent.UNSPECIFIED)
						.map((e) => (
							<Badge key={e} variant="outline">
								{EVENT_LABELS[e]}
							</Badge>
						))}
				</div>
			),
		},
		{
			id: "filters",
			header: "Filters",
			cell: ({ row }) => {
				const { namespaceFilter, pathPrefix } = row.original;
				if (!namespaceFilter && !pathPrefix) {
					return <span className="text-muted-foreground text-xs">All</span>;
				}
				return (
					<div className="flex flex-col gap-0.5 text-xs">
						{namespaceFilter && (
							<span>
								<span className="text-muted-foreground">ns: </span>
								{namespaceFilter}
							</span>
						)}
						{pathPrefix && (
							<span>
								<span className="text-muted-foreground">prefix: </span>
								{pathPrefix}
							</span>
						)}
					</div>
				);
			},
		},
		{
			id: "enabled",
			header: "Enabled",
			cell: ({ row }) => (
				<Checkbox
					checked={row.original.enabled}
					onCheckedChange={(checked) =>
						onToggleEnabled(row.original, !!checked)
					}
				/>
			),
		},
		{
			id: "actions",
			header: "",
			cell: ({ row }) => (
				<div className="flex items-center justify-end gap-1">
					<Button
						variant="ghost"
						size="icon-xs"
						title="Delivery history"
						render={<Link to={`/webhooks/${row.original.id}/history`} />}
					>
						<History className="h-3.5 w-3.5" />
					</Button>
					<Button
						variant="ghost"
						size="icon-xs"
						title="Edit webhook"
						onClick={() => onEdit(row.original)}
					>
						<Pencil className="h-3.5 w-3.5" />
					</Button>
					<DeleteDialog
						webhookId={row.original.id}
						webhookUrl={row.original.url}
					/>
				</div>
			),
		},
	];
}

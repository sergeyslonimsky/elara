import { useMutation } from "@connectrpc/connect-query";
import { useQueryClient } from "@tanstack/react-query";
import { Trash2 } from "lucide-react";
import { toast } from "sonner";
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
import { deleteWebhook } from "@/gen/elara/webhook/v1/webhook_service-WebhookService_connectquery";
import { invalidate } from "@/lib/queries";
import { toastError } from "@/lib/toast";

interface DeleteDialogProps {
	readonly webhookId: string;
	readonly webhookUrl: string;
}

export function DeleteDialog({ webhookId, webhookUrl }: DeleteDialogProps) {
	const queryClient = useQueryClient();

	const mutation = useMutation(deleteWebhook, {
		onSuccess: () => {
			toast.success("Webhook deleted");
			invalidate(queryClient, "webhooks");
		},
		onError: toastError,
	});

	return (
		<AlertDialog>
			<AlertDialogTrigger
				render={
					<Button variant="ghost" size="icon-xs" title="Delete webhook" />
				}
			>
				<Trash2 className="h-3.5 w-3.5 text-destructive" />
			</AlertDialogTrigger>
			<AlertDialogContent>
				<AlertDialogHeader>
					<AlertDialogTitle>Delete webhook?</AlertDialogTitle>
					<AlertDialogDescription>
						This will permanently delete the webhook pointing to{" "}
						<span className="font-medium break-all">{webhookUrl}</span>.
						Delivery history will be cleared.
					</AlertDialogDescription>
				</AlertDialogHeader>
				<AlertDialogFooter>
					<AlertDialogCancel>Cancel</AlertDialogCancel>
					<AlertDialogAction
						className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
						disabled={mutation.isPending}
						onClick={() => mutation.mutate({ id: webhookId })}
					>
						{mutation.isPending ? "Deleting..." : "Delete"}
					</AlertDialogAction>
				</AlertDialogFooter>
			</AlertDialogContent>
		</AlertDialog>
	);
}

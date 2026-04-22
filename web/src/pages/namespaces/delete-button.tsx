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
import { deleteNamespace } from "@/gen/elara/namespace/v1/namespace_service-NamespaceService_connectquery";
import { invalidate } from "@/lib/queries";
import { toastError } from "@/lib/toast";

export function DeleteButton({
	name,
	locked,
}: Readonly<{
	name: string;
	locked: boolean;
}>) {
	const queryClient = useQueryClient();

	const mutation = useMutation(deleteNamespace, {
		onSuccess: () => {
			toast.success(`Namespace "${name}" deleted`);
			invalidate(queryClient, "namespaces");
		},
		onError: toastError,
	});

	return (
		<AlertDialog>
			<AlertDialogTrigger
				render={
					<Button
						variant="ghost"
						size="icon-xs"
						disabled={locked ? true : undefined}
						title={locked ? `Namespace "${name}" is locked` : undefined}
					/>
				}
			>
				<Trash2 className="h-3.5 w-3.5 text-destructive" />
			</AlertDialogTrigger>
			<AlertDialogContent>
				<AlertDialogHeader>
					<AlertDialogTitle>Delete namespace "{name}"?</AlertDialogTitle>
					<AlertDialogDescription>
						This action cannot be undone. The namespace must be empty (no
						configs).
					</AlertDialogDescription>
				</AlertDialogHeader>
				<AlertDialogFooter>
					<AlertDialogCancel>Cancel</AlertDialogCancel>
					<AlertDialogAction
						className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
						disabled={mutation.isPending}
						onClick={() => mutation.mutate({ name })}
					>
						{mutation.isPending ? "Deleting..." : "Delete"}
					</AlertDialogAction>
				</AlertDialogFooter>
			</AlertDialogContent>
		</AlertDialog>
	);
}

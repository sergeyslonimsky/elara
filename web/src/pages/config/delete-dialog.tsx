import { useMutation } from "@connectrpc/connect-query";
import { useQueryClient } from "@tanstack/react-query";
import { Trash2 } from "lucide-react";
import { useNavigate } from "react-router";
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
import { deleteConfig } from "@/gen/elara/config/v1/config_service-ConfigService_connectquery";
import { useBackLink } from "@/hooks/use-back-link";
import { invalidateAllConfigData } from "@/lib/queries";
import { toastError } from "@/lib/toast";

interface DeleteDialogProps {
	path: string;
	namespace: string;
	effectiveLocked: boolean;
}

export function DeleteDialog({
	path,
	namespace,
	effectiveLocked,
}: DeleteDialogProps) {
	const navigate = useNavigate();
	const queryClient = useQueryClient();
	const backLink = useBackLink(namespace, path);

	const mutation = useMutation(deleteConfig, {
		onSuccess: () => {
			toast.success("Config deleted");
			void invalidateAllConfigData(queryClient);
			navigate(backLink);
		},
		onError: toastError,
	});

	return (
		<AlertDialog>
			<AlertDialogTrigger
				render={
					<Button
						variant="destructive"
						size="sm"
						disabled={effectiveLocked ? true : undefined}
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
						This will permanently delete <strong>{path}</strong> from namespace{" "}
						<strong>{namespace}</strong>.
					</AlertDialogDescription>
				</AlertDialogHeader>
				<AlertDialogFooter>
					<AlertDialogCancel>Cancel</AlertDialogCancel>
					<AlertDialogAction
						className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
						disabled={mutation.isPending}
						onClick={() => mutation.mutate({ path, namespace })}
					>
						{mutation.isPending ? "Deleting..." : "Delete"}
					</AlertDialogAction>
				</AlertDialogFooter>
			</AlertDialogContent>
		</AlertDialog>
	);
}

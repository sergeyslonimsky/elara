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
import { revokeToken } from "@/gen/elara/token/v1/token_service-TokenService_connectquery";
import { invalidate } from "@/lib/queries";
import { toastError } from "@/lib/toast";

interface RevokeTokenDialogProps {
	readonly tokenId: string;
	readonly tokenName: string;
}

export function RevokeTokenDialog({
	tokenId,
	tokenName,
}: RevokeTokenDialogProps) {
	const queryClient = useQueryClient();

	const mutation = useMutation(revokeToken, {
		onSuccess: () => {
			toast.success(`Token "${tokenName}" revoked`);
			invalidate(queryClient, "tokens");
		},
		onError: toastError,
	});

	return (
		<AlertDialog>
			<AlertDialogTrigger
				render={<Button variant="ghost" size="icon-xs" title="Revoke token" />}
			>
				<Trash2 className="h-3.5 w-3.5 text-destructive" />
				<span className="sr-only">Revoke token</span>
			</AlertDialogTrigger>
			<AlertDialogContent>
				<AlertDialogHeader>
					<AlertDialogTitle>Revoke token?</AlertDialogTitle>
					<AlertDialogDescription>
						This will immediately revoke the token{" "}
						<span className="font-medium">{tokenName}</span>. Clients
						authenticating with it will lose access. This action cannot be
						undone.
					</AlertDialogDescription>
				</AlertDialogHeader>
				<AlertDialogFooter>
					<AlertDialogCancel>Cancel</AlertDialogCancel>
					<AlertDialogAction
						className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
						disabled={mutation.isPending}
						onClick={() => mutation.mutate({ id: tokenId })}
					>
						{mutation.isPending ? "Revoking..." : "Revoke"}
					</AlertDialogAction>
				</AlertDialogFooter>
			</AlertDialogContent>
		</AlertDialog>
	);
}

import { useMutation } from "@connectrpc/connect-query";
import { useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import {
	Dialog,
	DialogContent,
	DialogDescription,
	DialogFooter,
	DialogHeader,
	DialogTitle,
} from "@/components/ui/dialog";
import type { User } from "@/gen/elara/user/v1/user_pb";
import { deactivateUser } from "@/gen/elara/user/v1/user_service-UserService_connectquery";
import { invalidate } from "@/lib/queries";
import { toastError } from "@/lib/toast";

interface DeactivateUserDialogProps {
	user: User;
	open: boolean;
	onOpenChange: (open: boolean) => void;
	onSuccess: () => void;
}

export function DeactivateUserDialog({
	user,
	open,
	onOpenChange,
	onSuccess,
}: Readonly<DeactivateUserDialogProps>) {
	const queryClient = useQueryClient();

	const mutation = useMutation(deactivateUser, {
		onSuccess: () => {
			toast.success(`User "${user.email}" deactivated`);
			invalidate(queryClient, "user");
			invalidate(queryClient, "users");
			onOpenChange(false);
			onSuccess();
		},
		onError: toastError,
	});

	const handleConfirm = () => {
		mutation.mutate({ userId: user.id });
	};

	return (
		<Dialog open={open} onOpenChange={onOpenChange}>
			<DialogContent>
				<DialogHeader>
					<DialogTitle>Deactivate User</DialogTitle>
					<DialogDescription>
						This will deactivate <strong>{user.email}</strong> and revoke all
						their active sessions. They will not be able to log in until
						reactivated.
					</DialogDescription>
				</DialogHeader>
				<DialogFooter>
					<Button
						variant="outline"
						onClick={() => onOpenChange(false)}
						disabled={mutation.isPending}
					>
						Cancel
					</Button>
					<Button
						variant="destructive"
						onClick={handleConfirm}
						disabled={mutation.isPending}
					>
						{mutation.isPending ? "Deactivating..." : "Deactivate user"}
					</Button>
				</DialogFooter>
			</DialogContent>
		</Dialog>
	);
}

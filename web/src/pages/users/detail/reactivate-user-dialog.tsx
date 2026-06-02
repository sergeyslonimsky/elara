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
import { reactivateUser } from "@/gen/elara/user/v1/user_service-UserService_connectquery";
import { invalidate } from "@/lib/queries";
import { toastError } from "@/lib/toast";

interface ReactivateUserDialogProps {
	user: User;
	open: boolean;
	onOpenChange: (open: boolean) => void;
	onSuccess: () => void;
}

export function ReactivateUserDialog({
	user,
	open,
	onOpenChange,
	onSuccess,
}: Readonly<ReactivateUserDialogProps>) {
	const queryClient = useQueryClient();

	const mutation = useMutation(reactivateUser, {
		onSuccess: () => {
			toast.success(`User "${user.email}" reactivated`);
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
					<DialogTitle>Reactivate User</DialogTitle>
					<DialogDescription>
						This will reactivate <strong>{user.email}</strong> and allow them to
						log in again.
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
					<Button onClick={handleConfirm} disabled={mutation.isPending}>
						{mutation.isPending ? "Reactivating..." : "Reactivate user"}
					</Button>
				</DialogFooter>
			</DialogContent>
		</Dialog>
	);
}

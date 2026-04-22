import { useMutation } from "@connectrpc/connect-query";
import { useQueryClient } from "@tanstack/react-query";
import { Lock, LockOpen } from "lucide-react";
import { useState } from "react";
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
import {
	lockNamespace,
	unlockNamespace,
} from "@/gen/elara/namespace/v1/namespace_service-NamespaceService_connectquery";
import { invalidate } from "@/lib/queries";
import { toastError } from "@/lib/toast";

export function LockButton({
	name,
	locked,
}: {
	name: string;
	locked: boolean;
}) {
	const queryClient = useQueryClient();
	const [open, setOpen] = useState(false);

	const lockMutation = useMutation(lockNamespace, {
		onSuccess: () => {
			toast.success(`Namespace "${name}" locked`);
			void invalidate(queryClient, "namespaces");
			setOpen(false);
		},
		onError: toastError,
	});

	const unlockMutation = useMutation(unlockNamespace, {
		onSuccess: () => {
			toast.success(`Namespace "${name}" unlocked`);
			void invalidate(queryClient, "namespaces");
			setOpen(false);
		},
		onError: toastError,
	});

	const isPending = lockMutation.isPending || unlockMutation.isPending;

	return (
		<AlertDialog open={open} onOpenChange={setOpen}>
			<AlertDialogTrigger render={<Button variant="outline" size="sm" />}>
				{locked ? (
					<>
						<LockOpen className="mr-1 h-3.5 w-3.5" />
						Unlock
					</>
				) : (
					<>
						<Lock className="mr-1 h-3.5 w-3.5" />
						Lock
					</>
				)}
			</AlertDialogTrigger>
			<AlertDialogContent>
				<AlertDialogHeader>
					<AlertDialogTitle>
						{locked
							? `Unlock namespace "${name}"?`
							: `Lock namespace "${name}"?`}
					</AlertDialogTitle>
					<AlertDialogDescription>
						{locked
							? "Unlocking restores all operations on this namespace and its configs."
							: "While locked, this namespace cannot be edited or deleted, and its configs cannot be created, updated, deleted, locked, or unlocked."}
					</AlertDialogDescription>
				</AlertDialogHeader>
				<AlertDialogFooter>
					<AlertDialogCancel>Cancel</AlertDialogCancel>
					<AlertDialogAction
						disabled={isPending}
						onClick={() =>
							locked
								? unlockMutation.mutate({ name })
								: lockMutation.mutate({ name })
						}
					>
						{locked
							? unlockMutation.isPending
								? "Unlocking..."
								: "Unlock"
							: lockMutation.isPending
								? "Locking..."
								: "Lock"}
					</AlertDialogAction>
				</AlertDialogFooter>
			</AlertDialogContent>
		</AlertDialog>
	);
}

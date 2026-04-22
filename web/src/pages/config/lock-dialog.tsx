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
	lockConfig,
	unlockConfig,
} from "@/gen/elara/config/v1/config_service-ConfigService_connectquery";
import { invalidateAllConfigData } from "@/lib/queries";
import { toastError } from "@/lib/toast";

interface LockDialogProps {
	path: string;
	namespace: string;
	configLocked: boolean;
	namespaceLocked: boolean;
}

function lockActionLabel(
	locked: boolean,
	lockPending: boolean,
	unlockPending: boolean,
): string {
	if (locked) return unlockPending ? "Unlocking..." : "Unlock";
	return lockPending ? "Locking..." : "Lock";
}

export function LockDialog({
	path,
	namespace,
	configLocked,
	namespaceLocked,
}: Readonly<LockDialogProps>) {
	const queryClient = useQueryClient();
	const [open, setOpen] = useState(false);

	const lockMutation = useMutation(lockConfig, {
		onSuccess: () => {
			toast.success("Config locked");
			invalidateAllConfigData(queryClient);
			setOpen(false);
		},
		onError: toastError,
	});

	const unlockMutation = useMutation(unlockConfig, {
		onSuccess: () => {
			toast.success("Config unlocked");
			invalidateAllConfigData(queryClient);
			setOpen(false);
		},
		onError: toastError,
	});

	return (
		<AlertDialog open={open} onOpenChange={setOpen}>
			<AlertDialogTrigger
				render={
					<Button
						variant="outline"
						size="sm"
						disabled={namespaceLocked ? true : undefined}
						title={namespaceLocked ? "Namespace is locked" : undefined}
					/>
				}
			>
				{configLocked ? (
					<>
						<LockOpen className="mr-1 h-4 w-4" />
						Unlock
					</>
				) : (
					<>
						<Lock className="mr-1 h-4 w-4" />
						Lock
					</>
				)}
			</AlertDialogTrigger>
			<AlertDialogContent>
				<AlertDialogHeader>
					<AlertDialogTitle>
						{configLocked ? "Unlock config?" : "Lock config?"}
					</AlertDialogTitle>
					<AlertDialogDescription>
						{configLocked
							? "Unlocking will allow this config to be updated or deleted."
							: "Locking will prevent this config from being updated or deleted until unlocked."}
					</AlertDialogDescription>
				</AlertDialogHeader>
				<AlertDialogFooter>
					<AlertDialogCancel>Cancel</AlertDialogCancel>
					<AlertDialogAction
						disabled={lockMutation.isPending || unlockMutation.isPending}
						onClick={() =>
							configLocked
								? unlockMutation.mutate({ path, namespace })
								: lockMutation.mutate({ path, namespace })
						}
					>
						{lockActionLabel(
							configLocked,
							lockMutation.isPending,
							unlockMutation.isPending,
						)}
					</AlertDialogAction>
				</AlertDialogFooter>
			</AlertDialogContent>
		</AlertDialog>
	);
}

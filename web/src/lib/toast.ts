import { toast } from "sonner";

/**
 * Default mutation error handler — pass as `onError` for `useMutation`:
 *
 *   useMutation(fn, { onError: toastError })
 *
 * Mutation errors in this app are always ConnectError-shaped; their `.message`
 * is already a user-readable code + message (e.g. "[not_found] foo").
 */
export function toastError(err: Error): void {
	toast.error(err.message);
}

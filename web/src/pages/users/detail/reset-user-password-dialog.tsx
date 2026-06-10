import { useMutation } from "@connectrpc/connect-query";
import { zodResolver } from "@hookform/resolvers/zod";
import { useEffect } from "react";
import { useForm } from "react-hook-form";
import { toast } from "sonner";
import { z } from "zod";
import { Button } from "@/components/ui/button";
import {
	Dialog,
	DialogContent,
	DialogDescription,
	DialogFooter,
	DialogHeader,
	DialogTitle,
} from "@/components/ui/dialog";
import { Field, FieldLabel } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import type { User } from "@/gen/elara/user/v1/user_pb";
import { resetUserPassword } from "@/gen/elara/user/v1/user_service-UserService_connectquery";
import { toastError } from "@/lib/toast";

const MIN_PASSWORD_LENGTH = 8;

const schema = z.object({
	newPassword: z
		.string()
		.min(
			MIN_PASSWORD_LENGTH,
			`Password must be at least ${MIN_PASSWORD_LENGTH} characters`,
		),
});

type FormValues = z.infer<typeof schema>;

interface ResetUserPasswordDialogProps {
	user: User;
	open: boolean;
	onOpenChange: (open: boolean) => void;
}

export function ResetUserPasswordDialog({
	user,
	open,
	onOpenChange,
}: Readonly<ResetUserPasswordDialogProps>) {
	const form = useForm<FormValues>({
		resolver: zodResolver(schema),
		defaultValues: { newPassword: "" },
		mode: "onChange",
	});

	useEffect(() => {
		if (!open) form.reset({ newPassword: "" });
	}, [open, form]);

	const mutation = useMutation(resetUserPassword, {
		onSuccess: () => {
			toast.success("Password reset");
			onOpenChange(false);
		},
		onError: toastError,
	});

	const onSubmit = (values: FormValues) => {
		mutation.mutate({ userId: user.id, newPassword: values.newPassword });
	};

	const newPasswordError = form.formState.errors.newPassword?.message;

	return (
		<Dialog open={open} onOpenChange={onOpenChange}>
			<DialogContent>
				<form onSubmit={form.handleSubmit(onSubmit)}>
					<DialogHeader>
						<DialogTitle>Reset Password</DialogTitle>
						<DialogDescription>
							This will force <em>{user.email}</em> to change their password on
							next login.
						</DialogDescription>
					</DialogHeader>
					<div className="grid gap-4 py-4">
						<Field>
							<FieldLabel htmlFor="new-password">New password</FieldLabel>
							<Input
								id="new-password"
								type="password"
								placeholder={`At least ${MIN_PASSWORD_LENGTH} characters`}
								{...form.register("newPassword")}
							/>
							{newPasswordError && (
								<p className="text-xs text-destructive">{newPasswordError}</p>
							)}
						</Field>
					</div>
					<DialogFooter>
						<Button
							type="submit"
							disabled={!form.formState.isValid || mutation.isPending}
						>
							{mutation.isPending ? "Resetting..." : "Reset password"}
						</Button>
					</DialogFooter>
				</form>
			</DialogContent>
		</Dialog>
	);
}

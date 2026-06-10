import { useMutation } from "@connectrpc/connect-query";
import { zodResolver } from "@hookform/resolvers/zod";
import { useQueryClient } from "@tanstack/react-query";
import { useEffect } from "react";
import { useForm } from "react-hook-form";
import { useNavigate } from "react-router";
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
import { Input } from "@/components/ui/input";
import type { User } from "@/gen/elara/user/v1/user_pb";
import { deleteUser } from "@/gen/elara/user/v1/user_service-UserService_connectquery";
import { invalidateUserGroupGraph } from "@/lib/queries";
import { toastError } from "@/lib/toast";

interface DeleteUserDialogProps {
	user: User;
	open: boolean;
	onOpenChange: (open: boolean) => void;
}

function makeSchema(expected: string) {
	return z.object({
		confirm: z
			.string()
			.transform((v) => v.trim())
			.refine((v) => v === expected, "Email does not match"),
	});
}

export function DeleteUserDialog({
	user,
	open,
	onOpenChange,
}: Readonly<DeleteUserDialogProps>) {
	const navigate = useNavigate();
	const queryClient = useQueryClient();

	const schema = makeSchema(user.email);
	type FormValues = z.infer<typeof schema>;

	const form = useForm<FormValues>({
		resolver: zodResolver(schema),
		defaultValues: { confirm: "" },
		mode: "onChange",
	});

	useEffect(() => {
		if (!open) form.reset({ confirm: "" });
	}, [open, form]);

	const mutation = useMutation(deleteUser, {
		onSuccess: () => {
			toast.success(`User "${user.email}" deleted`);
			invalidateUserGroupGraph(queryClient);
			onOpenChange(false);
			navigate("/users");
		},
		onError: toastError,
	});

	const onSubmit = () => {
		mutation.mutate({ userId: user.id });
	};

	return (
		<Dialog open={open} onOpenChange={onOpenChange}>
			<DialogContent>
				<form onSubmit={form.handleSubmit(onSubmit)}>
					<DialogHeader>
						<DialogTitle>Delete User</DialogTitle>
						<DialogDescription>
							This action is irreversible. Type <strong>{user.email}</strong> to
							confirm.
						</DialogDescription>
					</DialogHeader>
					<div className="py-4">
						<Input
							placeholder={user.email}
							aria-label="Confirm email"
							{...form.register("confirm")}
						/>
					</div>
					<DialogFooter>
						<Button
							type="submit"
							variant="destructive"
							disabled={!form.formState.isValid || mutation.isPending}
						>
							{mutation.isPending ? "Deleting..." : "Delete user"}
						</Button>
					</DialogFooter>
				</form>
			</DialogContent>
		</Dialog>
	);
}

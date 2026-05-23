import { useMutation } from "@connectrpc/connect-query";
import { useQueryClient } from "@tanstack/react-query";
import { Plus } from "lucide-react";
import { useMemo, useState } from "react";
import { toast } from "sonner";
import { useAuth } from "@/components/auth-provider";
import { Button } from "@/components/ui/button";
import {
	Dialog,
	DialogContent,
	DialogFooter,
	DialogHeader,
	DialogTitle,
	DialogTrigger,
} from "@/components/ui/dialog";
import { Field, FieldLabel } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { AuthType } from "@/gen/elara/auth/v1/auth_pb";
import { createUser } from "@/gen/elara/user/v1/user_service-UserService_connectquery";
import { invalidate } from "@/lib/queries";
import { toastError } from "@/lib/toast";

const MIN_PASSWORD_LENGTH = 8;
const EMAIL_REGEX = /^.+@.+\..+$/;

interface CreateUserFormValues {
	email: string;
	name: string;
	initialPassword: string;
}

export function validateCreateUserForm(
	values: CreateUserFormValues,
	authType: AuthType,
): Record<string, string> {
	const errors: Record<string, string> = {};

	const email = values.email.trim();
	if (!email) {
		errors.email = "Email is required";
	} else if (!EMAIL_REGEX.test(email)) {
		errors.email = "Invalid email format";
	}

	if (!values.name.trim()) {
		errors.name = "Name is required";
	}

	if (authType === AuthType.BASIC) {
		if (!values.initialPassword) {
			errors.initialPassword = "Password is required";
		} else if (values.initialPassword.length < MIN_PASSWORD_LENGTH) {
			errors.initialPassword = `Password must be at least ${MIN_PASSWORD_LENGTH} characters`;
		}
	}

	return errors;
}

export function CreateUserDialog() {
	const { state } = useAuth();
	const [open, setOpen] = useState(false);
	const [email, setEmail] = useState("");
	const [name, setName] = useState("");
	const [initialPassword, setInitialPassword] = useState("");
	const queryClient = useQueryClient();

	const canCreate =
		state.status === "authenticated" && state.ability.can("create", "User");
	const authType =
		state.status === "authenticated" || state.status === "anonymous"
			? state.authType
			: AuthType.UNSPECIFIED;
	const isBasicAuth = authType === AuthType.BASIC;

	const errors = useMemo(
		() => validateCreateUserForm({ email, name, initialPassword }, authType),
		[email, name, initialPassword, authType],
	);
	const hasErrors = Object.keys(errors).length > 0;

	const mutation = useMutation(createUser, {
		onSuccess: () => {
			toast.success(`User "${email}" created`);
			setOpen(false);
			setEmail("");
			setName("");
			setInitialPassword("");
			invalidate(queryClient, "users");
		},
		onError: toastError,
	});

	if (!canCreate) {
		return null;
	}

	return (
		<Dialog open={open} onOpenChange={setOpen}>
			<DialogTrigger render={<Button size="sm" />}>
				<Plus className="mr-1 h-4 w-4" />
				New User
			</DialogTrigger>
			<DialogContent>
				<form
					onSubmit={(e) => {
						e.preventDefault();
						if (hasErrors) return;
						mutation.mutate({
							email: email.trim(),
							name: name.trim(),
							initialPassword: isBasicAuth ? initialPassword : "",
						});
					}}
				>
					<DialogHeader>
						<DialogTitle>Create User</DialogTitle>
					</DialogHeader>
					<div className="grid gap-4 py-4">
						<Field>
							<FieldLabel>Email</FieldLabel>
							<Input
								type="email"
								value={email}
								onChange={(e) => setEmail(e.target.value)}
								placeholder="user@example.com"
								required
							/>
						</Field>
						<Field>
							<FieldLabel>Name</FieldLabel>
							<Input
								value={name}
								onChange={(e) => setName(e.target.value)}
								placeholder="Jane Doe"
								required
							/>
						</Field>
						{isBasicAuth && (
							<Field>
								<FieldLabel>Initial password</FieldLabel>
								<Input
									type="password"
									value={initialPassword}
									onChange={(e) => setInitialPassword(e.target.value)}
									placeholder={`At least ${MIN_PASSWORD_LENGTH} characters`}
									required
									minLength={MIN_PASSWORD_LENGTH}
								/>
							</Field>
						)}
					</div>
					<DialogFooter>
						<Button type="submit" disabled={mutation.isPending || hasErrors}>
							{mutation.isPending ? "Creating..." : "Create"}
						</Button>
					</DialogFooter>
				</form>
			</DialogContent>
		</Dialog>
	);
}

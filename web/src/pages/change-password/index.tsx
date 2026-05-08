import { ConnectError } from "@connectrpc/connect";
import { useMutation } from "@connectrpc/connect-query";
import { Key, LogOut } from "lucide-react";
import { useState } from "react";
import { useNavigate } from "react-router";
import { useAuth } from "@/components/auth-provider";
import { Button } from "@/components/ui/button";
import {
	Card,
	CardContent,
	CardDescription,
	CardHeader,
	CardTitle,
} from "@/components/ui/card";
import { Field, FieldLabel } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { changePassword } from "@/gen/elara/auth/v1/auth_service-AuthService_connectquery";

export function ChangePasswordPage() {
	const { me, logout } = useAuth();
	const navigate = useNavigate();
	const [currentPassword, setCurrentPassword] = useState("");
	const [newPassword, setNewPassword] = useState("");
	const [confirmPassword, setConfirmPassword] = useState("");
	const [error, setError] = useState<string | null>(null);

	const mutation = useMutation(changePassword);

	const onSubmit = async (e: React.FormEvent) => {
		e.preventDefault();
		setError(null);

		if (newPassword.length < 8) {
			setError("New password must be at least 8 characters");
			return;
		}

		if (newPassword !== confirmPassword) {
			setError("Passwords do not match");
			return;
		}

		try {
			await mutation.mutateAsync({
				currentPassword: currentPassword || undefined,
				newPassword: newPassword,
			});
			navigate("/");
		} catch (err) {
			const connectErr = ConnectError.from(err);
			setError(connectErr.message);
		}
	};

	return (
		<div className="flex min-h-screen items-center justify-center bg-muted/40 p-4">
			<Card className="w-full max-w-md">
				<CardHeader>
					<div className="flex items-center gap-2">
						<div className="flex h-8 w-8 items-center justify-center rounded-lg bg-primary/10 text-primary">
							<Key className="h-4 w-4" />
						</div>
						<CardTitle className="text-xl">Change Password</CardTitle>
					</div>
					{!me?.passwordChangeRequired && (
						<CardDescription>
							Update your password to keep your account secure.
						</CardDescription>
					)}
				</CardHeader>
				<CardContent>
					<form onSubmit={onSubmit} className="space-y-4">
						{!me?.passwordChangeRequired && (
							<Field>
								<FieldLabel>Current Password</FieldLabel>
								<Input
									type="password"
									value={currentPassword}
									onChange={(e) => setCurrentPassword(e.target.value)}
									autoComplete="current-password"
									required={!me?.passwordChangeRequired}
								/>
							</Field>
						)}
						<Field>
							<FieldLabel>New Password</FieldLabel>
							<Input
								type="password"
								value={newPassword}
								onChange={(e) => setNewPassword(e.target.value)}
								autoComplete="new-password"
								required
							/>
						</Field>
						<Field>
							<FieldLabel>Confirm New Password</FieldLabel>
							<Input
								type="password"
								value={confirmPassword}
								onChange={(e) => setConfirmPassword(e.target.value)}
								autoComplete="new-password"
								required
							/>
						</Field>

						{error && (
							<div className="text-sm font-medium text-destructive">
								{error}
							</div>
						)}

						<div className="flex flex-col gap-2 pt-2">
							<Button
								type="submit"
								className="w-full"
								disabled={
									mutation.isPending || !newPassword || !confirmPassword
								}
							>
								{mutation.isPending
									? "Changing password..."
									: "Change password"}
							</Button>
							<Button
								variant="ghost"
								type="button"
								className="w-full"
								onClick={() => logout()}
							>
								<LogOut className="mr-2 h-4 w-4" />
								Sign out
							</Button>
						</div>
					</form>
				</CardContent>
			</Card>
		</div>
	);
}

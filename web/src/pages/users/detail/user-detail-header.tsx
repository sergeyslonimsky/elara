import { KeyRound, Shield, Trash2, UserMinus, UserPlus } from "lucide-react";
import { useState } from "react";
import { useAbility } from "@/auth/ability-context";
import { ActionMenu } from "@/components/action-menu";
import { useAuth } from "@/components/auth-provider";
import { Badge } from "@/components/ui/badge";
import { AuthType } from "@/gen/elara/auth/v1/auth_pb";
import { type User, UserStatus } from "@/gen/elara/user/v1/user_pb";
import { DeactivateUserDialog } from "./deactivate-user-dialog";
import { DeleteUserDialog } from "./delete-user-dialog";
import { ReactivateUserDialog } from "./reactivate-user-dialog";
import { ResetUserPasswordDialog } from "./reset-user-password-dialog";

interface UserDetailHeaderProps {
	user: User;
	onRefetch: () => void;
}

export function UserDetailHeader({
	user,
	onRefetch,
}: Readonly<UserDetailHeaderProps>) {
	const { state } = useAuth();
	const ability = useAbility();
	const [resetPasswordOpen, setResetPasswordOpen] = useState(false);
	const [deleteOpen, setDeleteOpen] = useState(false);
	const [deactivateOpen, setDeactivateOpen] = useState(false);
	const [reactivateOpen, setReactivateOpen] = useState(false);

	const authType =
		state.status === "authenticated" || state.status === "anonymous"
			? state.authType
			: AuthType.UNSPECIFIED;

	const isBasicAuth = authType === AuthType.BASIC;
	const isSystem = user.isSystem;
	const isDeactivated = user.status === UserStatus.DEACTIVATED;
	const canWriteUser = ability.can("write", "User");

	const provider = user.identities?.[0]?.provider || "internal";

	return (
		<div className="flex items-start justify-between rounded-xl border bg-card p-4">
			<div className="flex flex-col gap-1">
				<div className="flex items-center gap-2">
					<h2 className="text-lg font-semibold">
						{user.displayName || user.email}
					</h2>
					{isSystem && (
						<Badge variant="secondary">
							<Shield className="mr-1 h-3 w-3" />
							System
						</Badge>
					)}
					{isDeactivated && <Badge variant="outline">Deactivated</Badge>}
				</div>
				<p className="font-mono text-sm text-muted-foreground">{user.email}</p>
				<p className="text-sm text-muted-foreground capitalize">{provider}</p>
			</div>

			{(() => {
				// Reset/Delete require basic-auth on the backend (requireBasicAuth in
				// handler.go) — meaningless in OIDC mode. Deactivate/Reactivate have no
				// such gate server-side (User:Write only), so they must not be hidden
				// behind isBasicAuth or OIDC admins can never deactivate a user via UI.
				// Delete additionally requires User:Write (service_manage.go's
				// authorizeUserWrite) — canWriteUser gates it too, same as
				// Deactivate/Reactivate, so a read-only basic-auth user doesn't see an
				// enabled action the backend will reject.
				const items = [
					...(isBasicAuth
						? [
								{
									label: "Reset password",
									icon: <KeyRound className="h-4 w-4" />,
									onClick: () => setResetPasswordOpen(true),
									disabled: isSystem,
								},
							]
						: []),
					...(canWriteUser
						? !isDeactivated
							? [
									{
										label: "Deactivate user",
										icon: <UserMinus className="h-4 w-4" />,
										onClick: () => setDeactivateOpen(true),
										variant: "destructive" as const,
										disabled: isSystem,
									},
								]
							: [
									{
										label: "Reactivate user",
										icon: <UserPlus className="h-4 w-4" />,
										onClick: () => setReactivateOpen(true),
										disabled: isSystem,
									},
								]
						: []),
					...(isBasicAuth && canWriteUser
						? [
								{
									label: "Delete user",
									icon: <Trash2 className="h-4 w-4" />,
									onClick: () => setDeleteOpen(true),
									variant: "destructive" as const,
									disabled: isSystem,
								},
							]
						: []),
				];
				return items.length > 0 ? (
					<ActionMenu label="User actions" items={items} />
				) : null;
			})()}

			<ResetUserPasswordDialog
				key={`reset-${user.email}`}
				user={user}
				open={resetPasswordOpen}
				onOpenChange={setResetPasswordOpen}
			/>
			<DeleteUserDialog
				key={`delete-${user.email}`}
				user={user}
				open={deleteOpen}
				onOpenChange={setDeleteOpen}
			/>
			<DeactivateUserDialog
				key={`deactivate-${user.email}`}
				user={user}
				open={deactivateOpen}
				onOpenChange={setDeactivateOpen}
				onSuccess={onRefetch}
			/>
			<ReactivateUserDialog
				key={`reactivate-${user.email}`}
				user={user}
				open={reactivateOpen}
				onOpenChange={setReactivateOpen}
				onSuccess={onRefetch}
			/>
		</div>
	);
}

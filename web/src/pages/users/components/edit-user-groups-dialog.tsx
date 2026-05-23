import { subject } from "@casl/ability";
import { useMutation, useQuery } from "@connectrpc/connect-query";
import { useQueryClient } from "@tanstack/react-query";
import { Lock, UsersRound } from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import { toast } from "sonner";
import { useAuth } from "@/components/auth-provider";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import {
	Dialog,
	DialogContent,
	DialogDescription,
	DialogFooter,
	DialogHeader,
	DialogTitle,
} from "@/components/ui/dialog";
import type { Group } from "@/gen/elara/group/v1/group_pb";
import { listGroups } from "@/gen/elara/group/v1/group_service-GroupService_connectquery";
import type { User } from "@/gen/elara/user/v1/user_pb";
import {
	getUser,
	updateUserGroups,
} from "@/gen/elara/user/v1/user_service-UserService_connectquery";
import { invalidate } from "@/lib/queries";
import { toastError } from "@/lib/toast";

interface EditUserGroupsDialogProps {
	user: User | null;
	open: boolean;
	onOpenChange: (open: boolean) => void;
}

export function EditUserGroupsDialog({
	user,
	open,
	onOpenChange,
}: EditUserGroupsDialogProps) {
	const { state } = useAuth();
	const queryClient = useQueryClient();

	const { data: userData, isLoading: isUserLoading } = useQuery(
		getUser,
		{ email: user?.email ?? "" },
		{ enabled: !!user && open },
	);

	const { data: groupsData, isLoading: isGroupsLoading } = useQuery(
		listGroups,
		{ pagination: { limit: 1000, offset: 0 } },
		{ enabled: open },
	);

	const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set());
	const [initialIds, setInitialIds] = useState<Set<string>>(new Set());

	useEffect(() => {
		if (open && userData) {
			const ids = new Set(userData.groupIds);
			setSelectedIds(ids);
			setInitialIds(ids);
		}
	}, [open, userData]);

	const mutation = useMutation(updateUserGroups, {
		onSuccess: () => {
			toast.success("User groups updated");
			onOpenChange(false);
			invalidate(queryClient, "user");
			invalidate(queryClient, "users");
		},
		onError: toastError,
	});

	const ability = state.status === "authenticated" ? state.ability : null;

	const allGroups = groupsData?.groups ?? [];

	// A group row is editable when the caller holds Group:Write on it. The
	// backend will additionally enforce anti-escalation (must hold every
	// permission the group grants) on additions, so writeable here is a
	// necessary-but-not-sufficient client hint — server stays authoritative.
	const isGroupEditable = (g: Group) =>
		ability?.can("write", subject("Group", { domain: `group:${g.name}` })) ??
		false;

	// The backend treats group_ids as the canonical full desired set, so we
	// must preserve memberships in groups the UI cannot touch — either
	// because they're outside the caller's list scope (not in allGroups) or
	// rendered read-only (visible but not editable). Only the
	// visible-and-editable slice is replaced from selectedIds.
	const buildSubmitGroupIds = (): string[] => {
		const touchable = new Set(
			allGroups.filter(isGroupEditable).map((g) => g.id),
		);
		const result = new Set<string>();
		for (const id of initialIds) {
			if (!touchable.has(id)) result.add(id);
		}
		for (const id of selectedIds) {
			if (touchable.has(id)) result.add(id);
		}
		return Array.from(result);
	};

	const handleSubmit = (e: React.FormEvent) => {
		e.preventDefault();
		if (!user) return;
		mutation.mutate({
			email: user.email,
			groupIds: buildSubmitGroupIds(),
		});
	};

	const toggle = (id: string) => {
		setSelectedIds((prev) => {
			const next = new Set(prev);
			if (next.has(id)) {
				next.delete(id);
			} else {
				next.add(id);
			}
			return next;
		});
	};

	const isLoading = isUserLoading || isGroupsLoading;
	const dirty = useMemo(() => {
		if (selectedIds.size !== initialIds.size) return true;
		for (const id of selectedIds) if (!initialIds.has(id)) return true;
		return false;
	}, [selectedIds, initialIds]);

	if (!user) return null;

	return (
		<Dialog open={open} onOpenChange={onOpenChange}>
			<DialogContent className="max-w-xl">
				<form onSubmit={handleSubmit}>
					<DialogHeader>
						<DialogTitle>Edit Groups: {user.email}</DialogTitle>
						<DialogDescription>
							Toggle group memberships. Groups you cannot manage are shown
							read-only and preserved on save.
						</DialogDescription>
					</DialogHeader>

					{isLoading ? (
						<div className="py-8 text-center text-muted-foreground">
							Loading...
						</div>
					) : (
						<div className="grid gap-2 py-4 max-h-[60vh] overflow-y-auto pr-2">
							{allGroups.length === 0 ? (
								<span className="text-sm text-muted-foreground italic">
									No groups available.
								</span>
							) : (
								allGroups.map((g) => {
									const editable = isGroupEditable(g);
									const checked = selectedIds.has(g.id);
									return (
										<label
											key={g.id}
											className="flex items-center justify-between p-2 border rounded-md hover:bg-muted/40 cursor-pointer"
										>
											<span className="flex items-center gap-2 text-sm">
												<Checkbox
													checked={checked}
													disabled={!editable}
													onCheckedChange={() => editable && toggle(g.id)}
												/>
												<UsersRound className="h-4 w-4 text-muted-foreground" />
												<span className="font-medium">{g.name}</span>
												{!editable && (
													<Badge variant="outline" className="text-[9px] h-4">
														<Lock className="h-2 w-2 mr-1 opacity-50" />
														READ ONLY
													</Badge>
												)}
											</span>
											<span className="text-xs text-muted-foreground font-mono">
												{g.id}
											</span>
										</label>
									);
								})
							)}
						</div>
					)}

					<DialogFooter className="mt-4">
						<Button
							type="button"
							variant="outline"
							onClick={() => onOpenChange(false)}
						>
							Cancel
						</Button>
						<Button
							type="submit"
							disabled={mutation.isPending || isLoading || !dirty}
						>
							{mutation.isPending ? "Saving..." : "Save Changes"}
						</Button>
					</DialogFooter>
				</form>
			</DialogContent>
		</Dialog>
	);
}

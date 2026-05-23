import { subject } from "@casl/ability";
import { useMutation, useQuery } from "@connectrpc/connect-query";
import { useQueryClient } from "@tanstack/react-query";
import { AlertCircle, Lock, Shield, Users } from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import { toast } from "sonner";
import { displayObject, subjectOf } from "@/auth/ability";
import { useAuth } from "@/components/auth-provider";
import { Badge } from "@/components/ui/badge";
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
import { Textarea } from "@/components/ui/textarea";
import {
	PermissionAction,
	type PermissionAssignment,
} from "@/gen/elara/common/v1/permission_pb";
import type { Group } from "@/gen/elara/group/v1/group_pb";
import {
	createGroup,
	deleteGroup,
	getGroup,
	updateGroup,
} from "@/gen/elara/group/v1/group_service-GroupService_connectquery";
import { invalidate } from "@/lib/queries";
import { toastError } from "@/lib/toast";

interface CreateGroupDialogProps {
	open: boolean;
	onOpenChange: (open: boolean) => void;
}

export function CreateGroupDialog({
	open,
	onOpenChange,
}: CreateGroupDialogProps) {
	const [name, setName] = useState("");
	const queryClient = useQueryClient();

	const mutation = useMutation(createGroup, {
		onSuccess: () => {
			toast.success(`Group "${name}" created`);
			onOpenChange(false);
			setName("");
			invalidate(queryClient, "groups");
		},
		onError: toastError,
	});

	return (
		<Dialog open={open} onOpenChange={onOpenChange}>
			<DialogContent>
				<form
					onSubmit={(e) => {
						e.preventDefault();
						mutation.mutate({ name });
					}}
				>
					<DialogHeader>
						<DialogTitle>Create Group</DialogTitle>
						<DialogDescription>
							Groups allow you to bundle permissions and assign them to multiple
							users.
						</DialogDescription>
					</DialogHeader>
					<div className="grid gap-4 py-4">
						<Field>
							<FieldLabel htmlFor="group-name">Name</FieldLabel>
							<Input
								id="group-name"
								value={name}
								onChange={(e) => setName(e.target.value)}
								placeholder="developers"
								required
							/>
						</Field>
					</div>
					<DialogFooter>
						<Button type="submit" disabled={mutation.isPending || !name}>
							{mutation.isPending ? "Creating..." : "Create"}
						</Button>
					</DialogFooter>
				</form>
			</DialogContent>
		</Dialog>
	);
}

interface EditGroupDialogProps {
	group: Group | null;
	open: boolean;
	onOpenChange: (open: boolean) => void;
}

export function EditGroupDialog({
	group,
	open,
	onOpenChange,
}: EditGroupDialogProps) {
	const { state } = useAuth();
	const queryClient = useQueryClient();

	const { data: fullGroupData, isLoading: isGroupLoading } = useQuery(
		getGroup,
		{ id: group?.id ?? "" },
		{ enabled: !!group && open },
	);

	const [name, setName] = useState("");
	const [description, setDescription] = useState("");
	const [members, setMembers] = useState<string[]>([]);
	const [memberInput, setMemberInput] = useState("");
	const [permissions, setPermissions] = useState<PermissionAssignment[]>([]);
	const [version, setVersion] = useState<bigint>(0n);

	useEffect(() => {
		if (open && fullGroupData?.group) {
			const g = fullGroupData.group;
			setName(g.name);
			setDescription(g.description);
			setMembers([...g.members]);
			setMemberInput("");
			setPermissions([...g.permissions]);
			setVersion(g.version);
		}
	}, [open, fullGroupData]);

	const mutation = useMutation(updateGroup, {
		onSuccess: () => {
			toast.success("Group updated");
			onOpenChange(false);
			invalidate(queryClient, "groups");
			invalidate(queryClient, "group");
		},
		onError: toastError,
	});

	const ability = state.status === "authenticated" ? state.ability : null;

	const groupDomain = useMemo(
		() => (group ? `group:${group.name}` : ""),
		[group],
	);
	const groupSubject = useMemo(
		() => (group ? subject("Group", { domain: groupDomain }) : null),
		[group, groupDomain],
	);

	const canEditMetadata = useMemo(
		() =>
			groupSubject ? (ability?.can("write", groupSubject) ?? false) : false,
		[ability, groupSubject],
	);

	/**
	 * Per-item boundary logic (§8).
	 * Note: This is an approximation. Backend will enforce the actual union boundary.
	 */
	const isMemberEditable = (_email: string) => {
		// Managing members in a group requires write access to that group
		return (groupSubject && ability?.can("write", groupSubject)) ?? false;
	};

	const isPermissionEditable = (p: PermissionAssignment) => {
		// Modifying a permission requires write access to the resource it governs (§8)
		const name = subjectOf(p.object);
		if (name === "all") {
			return ability?.can("write", "all") ?? false;
		}
		const sub = subject(name, { domain: p.domain });
		return ability?.can("write", sub) ?? false;
	};

	const formatAction = (action: PermissionAction) => {
		switch (action) {
			case PermissionAction.READ:
				return "READ";
			case PermissionAction.WRITE:
				return "WRITE";
			case PermissionAction.ALL:
				return "ALL";
			default:
				return "UNKNOWN";
		}
	};

	const handleSubmit = (e: React.FormEvent) => {
		e.preventDefault();
		const pending = memberInput.trim();
		const finalMembers =
			pending && !members.includes(pending) ? [...members, pending] : members;
		mutation.mutate({
			id: group?.id ?? "",
			name,
			description,
			members: finalMembers,
			permissions,
			version,
		});
	};

	if (!group) return null;

	return (
		<Dialog open={open} onOpenChange={onOpenChange}>
			<DialogContent className="max-w-2xl">
				<form onSubmit={handleSubmit}>
					<DialogHeader>
						<DialogTitle>Edit Group: {group.name}</DialogTitle>
						<DialogDescription>
							Update group details, members, and permissions.
						</DialogDescription>
					</DialogHeader>

					{isGroupLoading ? (
						<div className="py-8 text-center text-muted-foreground">
							Loading group data...
						</div>
					) : (
						<div className="grid gap-6 py-4 max-h-[70vh] overflow-y-auto pr-2">
							<div className="grid gap-4">
								<h3 className="text-sm font-semibold flex items-center gap-2">
									<AlertCircle className="h-4 w-4" /> General Info
								</h3>
								<Field>
									<FieldLabel htmlFor="edit-group-name">Name</FieldLabel>
									<Input
										id="edit-group-name"
										value={name}
										onChange={(e) => setName(e.target.value)}
										disabled={!canEditMetadata}
										required
									/>
								</Field>
								<Field>
									<FieldLabel htmlFor="edit-group-description">
										Description
									</FieldLabel>
									<Textarea
										id="edit-group-description"
										value={description}
										onChange={(e) => setDescription(e.target.value)}
										disabled={!canEditMetadata}
									/>
								</Field>
							</div>

							<div className="space-y-3">
								<h3 className="text-sm font-semibold flex items-center justify-between">
									<span className="flex items-center gap-2">
										<Users className="h-4 w-4" /> Members
									</span>
								</h3>
								<div className="flex flex-wrap gap-2 p-3 border rounded-md bg-muted/30">
									{members.length > 0 ? (
										members.map((email) => {
											const editable = isMemberEditable(email);
											return (
												<Badge
													key={email}
													variant={editable ? "secondary" : "outline"}
													className="flex items-center gap-1"
												>
													{!editable && (
														<Lock className="h-2 w-2 mr-1 opacity-50" />
													)}
													{email}
													{editable && (
														<button
															type="button"
															onClick={() =>
																setMembers(members.filter((m) => m !== email))
															}
															className="hover:text-destructive"
														>
															×
														</button>
													)}
												</Badge>
											);
										})
									) : (
										<span className="text-xs text-muted-foreground italic">
											No members in this group.
										</span>
									)}
								</div>
								{canEditMetadata && (
									<div className="flex gap-2">
										<Input
											placeholder="Add user email and press Enter..."
											className="h-8 text-xs"
											value={memberInput}
											onChange={(e) => setMemberInput(e.target.value)}
											onKeyDown={(e) => {
												if (e.key === "Enter") {
													e.preventDefault();
													const val = memberInput.trim();
													if (val && !members.includes(val)) {
														setMembers([...members, val]);
														setMemberInput("");
													}
												}
											}}
										/>
									</div>
								)}
							</div>

							<div className="space-y-3">
								<h3 className="text-sm font-semibold flex items-center justify-between">
									<span className="flex items-center gap-2">
										<Shield className="h-4 w-4" /> Permissions
									</span>
								</h3>
								<div className="space-y-2">
									{permissions.length > 0 ? (
										permissions.map((p, i) => {
											const editable = isPermissionEditable(p);
											return (
												<div
													key={`${p.object}-${p.domain}-${p.action}`}
													className="flex items-center justify-between p-2 text-xs border rounded-md bg-background"
												>
													<span className="flex items-center gap-2">
														{!editable && (
															<Lock className="h-3 w-3 text-muted-foreground" />
														)}
														<span
															className={
																editable ? "" : "text-muted-foreground"
															}
														>
															<strong>{displayObject(p.object)}</strong> in{" "}
															<strong>{p.domain}</strong> —{" "}
															{formatAction(p.action)}
														</span>
														{!editable && (
															<Badge
																variant="outline"
																className="text-[9px] h-4"
															>
																READ ONLY
															</Badge>
														)}
													</span>
													{editable && (
														<Button
															type="button"
															variant="ghost"
															size="xs"
															onClick={() =>
																setPermissions(
																	permissions.filter((_, idx) => idx !== i),
																)
															}
														>
															Remove
														</Button>
													)}
												</div>
											);
										})
									) : (
										<span className="text-xs text-muted-foreground italic px-2">
											No permissions assigned.
										</span>
									)}
								</div>
							</div>
						</div>
					)}

					<DialogFooter className="mt-4">
						<Button
							type="submit"
							disabled={
								mutation.isPending || isGroupLoading || !canEditMetadata
							}
						>
							{mutation.isPending ? "Saving..." : "Save Changes"}
						</Button>
					</DialogFooter>
				</form>
			</DialogContent>
		</Dialog>
	);
}

interface DeleteGroupDialogProps {
	group: Group | null;
	open: boolean;
	onOpenChange: (open: boolean) => void;
}

export function DeleteGroupDialog({
	group,
	open,
	onOpenChange,
}: DeleteGroupDialogProps) {
	const queryClient = useQueryClient();

	const mutation = useMutation(deleteGroup, {
		onSuccess: () => {
			toast.success("Group deleted");
			onOpenChange(false);
			invalidate(queryClient, "groups");
		},
		onError: toastError,
	});

	if (!group) return null;

	return (
		<Dialog open={open} onOpenChange={onOpenChange}>
			<DialogContent>
				<DialogHeader>
					<DialogTitle>Delete Group</DialogTitle>
					<DialogDescription>
						Are you sure you want to delete the group{" "}
						<strong>{group.name}</strong>? This action cannot be undone and will
						remove all associated permissions from its members.
					</DialogDescription>
				</DialogHeader>
				<DialogFooter>
					<Button variant="outline" onClick={() => onOpenChange(false)}>
						Cancel
					</Button>
					<Button
						variant="destructive"
						onClick={() => mutation.mutate({ id: group.id })}
						disabled={mutation.isPending}
					>
						{mutation.isPending ? "Deleting..." : "Delete Group"}
					</Button>
				</DialogFooter>
			</DialogContent>
		</Dialog>
	);
}

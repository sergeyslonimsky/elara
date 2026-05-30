import { create } from "@bufbuild/protobuf";
import { Code, ConnectError } from "@connectrpc/connect";
import { useMutation } from "@connectrpc/connect-query";
import { useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { toast } from "sonner";
import { canManageGroup, displayObject, formatAction } from "@/auth/ability";
import { useAbility } from "@/auth/ability-context";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
	Select,
	SelectContent,
	SelectItem,
	SelectTrigger,
	SelectValue,
} from "@/components/ui/select";
import {
	PermissionAction,
	type PermissionAssignment,
	PermissionAssignmentSchema,
	PermissionObject,
} from "@/gen/elara/common/v1/permission_pb";
import type { Group } from "@/gen/elara/group/v1/group_pb";
import { updateGroupPermissions } from "@/gen/elara/group/v1/group_service-GroupService_connectquery";
import { invalidate } from "@/lib/queries";
import { toastError } from "@/lib/toast";

interface PermissionsTabProps {
	group: Group;
	permissions: PermissionAssignment[];
}

function permKey(p: PermissionAssignment): string {
	return `${p.object}|${p.action}|${p.domain}`;
}

const PERMISSION_OBJECTS = [
	PermissionObject.NAMESPACE,
	PermissionObject.CLIENT,
	PermissionObject.USER,
	PermissionObject.GROUP,
	PermissionObject.TOKEN,
	PermissionObject.WEBHOOK,
];

const PERMISSION_ACTIONS = [
	PermissionAction.READ,
	PermissionAction.WRITE,
	PermissionAction.CREATE,
	PermissionAction.DELETE,
	PermissionAction.ALL,
];

export function PermissionsTab({
	group,
	permissions,
}: Readonly<PermissionsTabProps>) {
	const ability = useAbility();
	const queryClient = useQueryClient();

	const canEdit = canManageGroup(ability, group);

	const [addPerms, setAddPerms] = useState<PermissionAssignment[]>([]);
	const [removeKeys, setRemoveKeys] = useState<Set<string>>(new Set());

	const [newObject, setNewObject] = useState<PermissionObject>(
		PermissionObject.NAMESPACE,
	);
	const [newAction, setNewAction] = useState<PermissionAction>(
		PermissionAction.READ,
	);
	const [newDomain, setNewDomain] = useState("");

	const hasChanges = addPerms.length > 0 || removeKeys.size > 0;

	const mutation = useMutation(updateGroupPermissions, {
		onSuccess: () => {
			toast.success("Permissions updated");
			setAddPerms([]);
			setRemoveKeys(new Set());
			invalidate(queryClient, "group");
			invalidate(queryClient, "groups");
		},
		onError: (err) => {
			const connectErr = ConnectError.from(err);
			if (connectErr.code === Code.FailedPrecondition) {
				toast.error(
					"Group changed since you loaded the page — refresh to retry.",
				);
			} else {
				toastError(err);
			}
		},
	});

	const stagePerm = () => {
		const domain = newDomain.trim() || "*";
		const perm = create(PermissionAssignmentSchema, {
			object: newObject,
			action: newAction,
			domain,
		});
		const key = permKey(perm);
		const alreadyExists =
			permissions.some((p) => permKey(p) === key) ||
			addPerms.some((p) => permKey(p) === key);
		if (alreadyExists) return;
		setAddPerms([...addPerms, perm]);
		setNewDomain("");
	};

	const handleSave = () => {
		mutation.mutate({
			groupId: group.id,
			add: addPerms,
			remove: permissions.filter((p) => removeKeys.has(permKey(p))),
			expectedPermissionsVersion: group.permissionsVersion,
		});
	};

	return (
		<div className="mt-2 flex flex-col gap-3">
			<div className="rounded-xl border bg-card divide-y">
				{permissions.length === 0 && addPerms.length === 0 && (
					<p className="p-4 text-sm text-muted-foreground italic">
						No permissions assigned.
					</p>
				)}
				{permissions.map((p) => {
					const key = permKey(p);
					const isRemoved = removeKeys.has(key);
					return (
						<div
							key={key}
							className="flex items-center justify-between p-3 text-sm"
						>
							<span
								className={
									isRemoved ? "line-through text-muted-foreground" : ""
								}
							>
								<strong>{displayObject(p.object)}</strong> in{" "}
								<strong>{p.domain}</strong> — {formatAction(p.action)}
							</span>
							{canEdit &&
								(isRemoved ? (
									<Button
										type="button"
										variant="ghost"
										size="sm"
										onClick={() => {
											setRemoveKeys((prev) => {
												const next = new Set(prev);
												next.delete(key);
												return next;
											});
										}}
									>
										Undo
									</Button>
								) : (
									<Button
										type="button"
										variant="ghost"
										size="sm"
										className="text-destructive hover:text-destructive"
										onClick={() =>
											setRemoveKeys((prev) => new Set([...prev, key]))
										}
									>
										Remove
									</Button>
								))}
						</div>
					);
				})}
				{addPerms.map((p) => {
					const key = permKey(p);
					return (
						<div
							key={key}
							className="flex items-center justify-between p-3 text-sm text-emerald-500"
						>
							<span>
								+ <strong>{displayObject(p.object)}</strong> in{" "}
								<strong>{p.domain}</strong> — {formatAction(p.action)}
							</span>
							<Button
								type="button"
								variant="ghost"
								size="sm"
								aria-label={`Un-stage permission ${displayObject(p.object)} ${p.domain}`}
								onClick={() =>
									setAddPerms(addPerms.filter((a) => permKey(a) !== key))
								}
							>
								×
							</Button>
						</div>
					);
				})}
			</div>

			{canEdit && (
				<div className="rounded-xl border bg-card p-4">
					<p className="text-sm font-medium mb-2">Add permission</p>
					<div className="flex flex-wrap gap-2 items-end">
						<Select
							value={String(newObject)}
							onValueChange={(v) => setNewObject(Number(v) as PermissionObject)}
						>
							<SelectTrigger
								className="w-36 h-8 text-sm"
								aria-label="Permission object"
							>
								<SelectValue />
							</SelectTrigger>
							<SelectContent>
								{PERMISSION_OBJECTS.map((obj) => (
									<SelectItem key={obj} value={String(obj)}>
										{displayObject(obj)}
									</SelectItem>
								))}
							</SelectContent>
						</Select>
						<Select
							value={String(newAction)}
							onValueChange={(v) => setNewAction(Number(v) as PermissionAction)}
						>
							<SelectTrigger
								className="w-28 h-8 text-sm"
								aria-label="Permission action"
							>
								<SelectValue />
							</SelectTrigger>
							<SelectContent>
								{PERMISSION_ACTIONS.map((value) => (
									<SelectItem key={value} value={String(value)}>
										{formatAction(value)}
									</SelectItem>
								))}
							</SelectContent>
						</Select>
						<Input
							value={newDomain}
							onChange={(e) => setNewDomain(e.target.value)}
							placeholder="domain (blank = *)"
							aria-label="Permission domain"
							className="h-8 text-sm w-44"
						/>
						<Button
							type="button"
							size="sm"
							variant="outline"
							onClick={stagePerm}
						>
							Add
						</Button>
					</div>
				</div>
			)}

			{addPerms.length > 0 && (
				<div className="flex flex-wrap gap-1">
					{addPerms.map((p) => (
						<Badge
							key={permKey(p)}
							variant="outline"
							className="text-emerald-500"
						>
							+{displayObject(p.object)}:{p.domain}
						</Badge>
					))}
				</div>
			)}

			<div className="flex justify-end">
				<Button
					size="sm"
					onClick={handleSave}
					disabled={!canEdit || !hasChanges || mutation.isPending}
				>
					{mutation.isPending ? "Saving..." : "Save changes"}
				</Button>
			</div>
		</div>
	);
}

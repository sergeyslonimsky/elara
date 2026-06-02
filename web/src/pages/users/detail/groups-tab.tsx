import { Code, ConnectError } from "@connectrpc/connect";
import { useMutation, useQuery } from "@connectrpc/connect-query";
import { useQueryClient } from "@tanstack/react-query";
import { useMemo, useState } from "react";
import { toast } from "sonner";
import { canManageGroup } from "@/auth/ability";
import { useAbility } from "@/auth/ability-context";
import { SkeletonList } from "@/components/skeleton-list";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import type { Group } from "@/gen/elara/group/v1/group_pb";
import { listGroups } from "@/gen/elara/group/v1/group_service-GroupService_connectquery";
import type { User } from "@/gen/elara/user/v1/user_pb";
import { updateUserGroups } from "@/gen/elara/user/v1/user_service-UserService_connectquery";
import { invalidateUserGroupGraph } from "@/lib/queries";
import { toastError } from "@/lib/toast";

interface GroupsTabProps {
	user: User;
	visibleGroupIds: string[];
	membershipVersion: bigint;
}

export function GroupsTab({
	user,
	visibleGroupIds,
	membershipVersion,
}: Readonly<GroupsTabProps>) {
	const ability = useAbility();
	const queryClient = useQueryClient();

	const { data: groupsData, isLoading } = useQuery(listGroups, {
		pagination: { limit: 1000, offset: 0 },
	});

	const allGroups = groupsData?.groups ?? [];

	// Local delta state — independent of server data, so background refetches
	// do not blow away in-progress edits.
	const [addIds, setAddIds] = useState<Set<string>>(new Set());
	const [removeIds, setRemoveIds] = useState<Set<string>>(new Set());

	const visibleSet = useMemo(() => new Set(visibleGroupIds), [visibleGroupIds]);

	const isChecked = (id: string): boolean => {
		if (removeIds.has(id)) return false;
		if (addIds.has(id)) return true;
		return visibleSet.has(id);
	};

	const toggle = (g: Group) => {
		if (!canManageGroup(ability, g)) return;
		const id = g.name;
		const isMember = visibleSet.has(id);
		if (isMember) {
			// Currently a member → toggle by staging a remove (or un-staging).
			setRemoveIds((prev) => {
				const next = new Set(prev);
				if (next.has(id)) next.delete(id);
				else next.add(id);
				return next;
			});
		} else {
			// Not currently a member → toggle by staging an add (or un-staging).
			setAddIds((prev) => {
				const next = new Set(prev);
				if (next.has(id)) next.delete(id);
				else next.add(id);
				return next;
			});
		}
	};

	const hasChanges = addIds.size > 0 || removeIds.size > 0;

	const mutation = useMutation(updateUserGroups, {
		onSuccess: () => {
			toast.success("Group memberships updated");
			setAddIds(new Set());
			setRemoveIds(new Set());
			invalidateUserGroupGraph(queryClient);
		},
		onError: (err) => {
			const connectErr = ConnectError.from(err);
			if (connectErr.code === Code.FailedPrecondition) {
				toast.error(
					"Memberships changed since you loaded the page — refresh to retry.",
				);
			} else {
				toastError(err);
			}
		},
	});

	const handleSave = () => {
		if (!hasChanges) return;
		mutation.mutate({
			userId: user.id,
			addGroupIds: Array.from(addIds),
			removeGroupIds: Array.from(removeIds),
			expectedVersion: membershipVersion,
		});
	};

	if (isLoading) {
		return <SkeletonList count={4} className="h-10 mt-2" />;
	}

	return (
		<div className="mt-2 flex flex-col gap-3">
			<div className="rounded-xl border bg-card divide-y">
				{allGroups.length === 0 && (
					<p className="p-4 text-sm text-muted-foreground">
						No groups available.
					</p>
				)}
				{allGroups.map((g) => {
					const editable = canManageGroup(ability, g);
					const checked = isChecked(g.name);
					const checkboxId = `group-cb-${g.name}`;
					return (
						<div
							key={g.name}
							className="flex items-center justify-between gap-3 p-3"
						>
							<div className="flex items-center gap-3">
								<Checkbox
									id={checkboxId}
									checked={checked}
									disabled={!editable}
									onCheckedChange={() => toggle(g)}
									aria-label={g.name}
								/>
								<div className="flex flex-col">
									<label
										htmlFor={checkboxId}
										className="text-sm font-medium cursor-pointer"
									>
										{g.displayName || g.name}
									</label>
									<span className="font-mono text-xs text-muted-foreground">
										{g.name}
									</span>
								</div>
							</div>
							{!editable && (
								<Badge variant="outline" className="text-xs">
									READ ONLY
								</Badge>
							)}
						</div>
					);
				})}
			</div>
			<p className="text-xs text-muted-foreground">
				Memberships outside your read scope are preserved on save.
			</p>
			<div className="flex justify-end">
				<Button
					size="sm"
					onClick={handleSave}
					disabled={!hasChanges || mutation.isPending}
				>
					{mutation.isPending ? "Saving..." : "Save changes"}
				</Button>
			</div>
		</div>
	);
}

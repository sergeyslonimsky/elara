import { Code, ConnectError } from "@connectrpc/connect";
import { useMutation } from "@connectrpc/connect-query";
import { useQueryClient } from "@tanstack/react-query";
import { useMemo, useState } from "react";
import { toast } from "sonner";
import { canManageGroup } from "@/auth/ability";
import { useAbility } from "@/auth/ability-context";
import { UserFilter } from "@/components/filters";
import { Button } from "@/components/ui/button";
import type { Group } from "@/gen/elara/group/v1/group_pb";
import { updateGroupMembers } from "@/gen/elara/group/v1/group_service-GroupService_connectquery";
import { invalidateUserGroupGraph } from "@/lib/queries";
import { toastError } from "@/lib/toast";

interface PendingChanges {
	adds: string[];
	removes: string[];
}

const EMPTY_PENDING: PendingChanges = { adds: [], removes: [] };

interface MembersTabProps {
	group: Group;
	visibleMembers: string[];
}

export function MembersTab({
	group,
	visibleMembers,
}: Readonly<MembersTabProps>) {
	const ability = useAbility();
	const queryClient = useQueryClient();

	const canEdit = canManageGroup(ability, group);

	const [pending, setPending] = useState<PendingChanges>(EMPTY_PENDING);

	const currentMembers = useMemo(() => {
		const removed = new Set(pending.removes);
		return [...visibleMembers.filter((e) => !removed.has(e)), ...pending.adds];
	}, [visibleMembers, pending]);

	const hasChanges = pending.adds.length > 0 || pending.removes.length > 0;

	const mutation = useMutation(updateGroupMembers, {
		onSuccess: () => {
			toast.success("Members updated");
			setPending(EMPTY_PENDING);
			invalidateUserGroupGraph(queryClient);
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

	const handleMembersChange = (next: string[]) => {
		const nextSet = new Set(next);
		const visibleSet = new Set(visibleMembers);
		setPending({
			adds: next.filter((e) => !visibleSet.has(e)),
			removes: visibleMembers.filter((e) => !nextSet.has(e)),
		});
	};

	const handleSave = () => {
		mutation.mutate({
			groupName: group.name,
			addEmails: pending.adds,
			removeEmails: pending.removes,
			expectedMembersVersion: group.membersVersion,
		});
	};

	return (
		<div className="mt-2 flex flex-col gap-3">
			<div className="rounded-xl border bg-card p-4">
				<p className="text-sm text-muted-foreground mb-3">
					{visibleMembers.length} member(s) visible to you.
				</p>

				{canEdit ? (
					<UserFilter
						value={currentMembers}
						onValueChange={handleMembersChange}
					/>
				) : (
					<p className="text-sm text-muted-foreground italic">
						You don't have permission to edit this group's members.
					</p>
				)}
			</div>

			<p className="text-xs text-muted-foreground">
				Members outside your read scope are preserved on save.
			</p>

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

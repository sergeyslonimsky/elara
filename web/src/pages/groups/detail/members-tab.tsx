import { Code, ConnectError } from "@connectrpc/connect";
import { useMutation } from "@connectrpc/connect-query";
import { useQueryClient } from "@tanstack/react-query";
import { X } from "lucide-react";
import { useState } from "react";
import { toast } from "sonner";
import { z } from "zod";
import { canManageGroup } from "@/auth/ability";
import { useAuth } from "@/components/auth-provider";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import type { Group } from "@/gen/elara/group/v1/group_pb";
import { updateGroupMembers } from "@/gen/elara/group/v1/group_service-GroupService_connectquery";
import { invalidateUserGroupGraph } from "@/lib/queries";
import { toastError } from "@/lib/toast";

const emailSchema = z.string().email();

interface MembersTabProps {
	group: Group;
	visibleMembers: string[];
}

export function MembersTab({
	group,
	visibleMembers,
}: Readonly<MembersTabProps>) {
	const { state } = useAuth();
	const queryClient = useQueryClient();

	const ability = state.status === "authenticated" ? state.ability : null;
	const canEdit = ability ? canManageGroup(ability, group) : false;

	const [addEmails, setAddEmails] = useState<string[]>([]);
	const [removeEmails, setRemoveEmails] = useState<Set<string>>(new Set());
	const [draft, setDraft] = useState("");
	const [draftError, setDraftError] = useState<string | null>(null);

	const hasChanges = addEmails.length > 0 || removeEmails.size > 0;

	const mutation = useMutation(updateGroupMembers, {
		onSuccess: () => {
			toast.success("Members updated");
			setAddEmails([]);
			setRemoveEmails(new Set());
			setDraft("");
			setDraftError(null);
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

	const stageDraftAdd = () => {
		const val = draft.trim();
		if (!val) {
			setDraftError(null);
			return;
		}
		const parsed = emailSchema.safeParse(val);
		if (!parsed.success) {
			setDraftError("Invalid email");
			return;
		}
		if (addEmails.includes(val)) {
			setDraftError("Already staged for add");
			return;
		}
		if (visibleMembers.includes(val)) {
			setDraftError("Already a member");
			return;
		}
		setAddEmails([...addEmails, val]);
		setDraft("");
		setDraftError(null);
	};

	const stageRemove = (email: string) => {
		setRemoveEmails((prev) => new Set([...prev, email]));
	};

	const unstageRemove = (email: string) => {
		setRemoveEmails((prev) => {
			const next = new Set(prev);
			next.delete(email);
			return next;
		});
	};

	const handleSave = () => {
		mutation.mutate({
			groupId: group.id,
			addEmails,
			removeEmails: Array.from(removeEmails),
			expectedMembersVersion: group.membersVersion,
		});
	};

	return (
		<div className="mt-2 flex flex-col gap-3">
			<div className="rounded-xl border bg-card p-4">
				<p className="text-sm text-muted-foreground mb-3">
					{visibleMembers.length} member(s) visible to you.
				</p>

				<div className="flex flex-wrap gap-2 mb-3">
					{visibleMembers.map((email) => {
						const isStaged = removeEmails.has(email);
						return (
							<Badge
								key={email}
								variant={isStaged ? "destructive" : "secondary"}
								className={isStaged ? "line-through opacity-60" : ""}
							>
								{email}
								{canEdit &&
									(isStaged ? (
										<button
											type="button"
											aria-label={`Undo remove ${email}`}
											className="ml-1 hover:text-primary"
											onClick={() => unstageRemove(email)}
										>
											↩
										</button>
									) : (
										<button
											type="button"
											aria-label={`Remove ${email}`}
											className="ml-1 hover:text-destructive"
											onClick={() => stageRemove(email)}
										>
											<X className="h-3 w-3" />
										</button>
									))}
							</Badge>
						);
					})}
					{visibleMembers.length === 0 && (
						<span className="text-sm text-muted-foreground italic">
							No visible members.
						</span>
					)}
				</div>

				{addEmails.length > 0 && (
					<div className="mb-3">
						<p className="text-xs text-muted-foreground mb-1">To be added:</p>
						<div className="flex flex-wrap gap-2">
							{addEmails.map((email) => (
								<Badge
									key={email}
									variant="outline"
									className="text-emerald-500"
								>
									+{email}
									<button
										type="button"
										aria-label={`Un-stage ${email}`}
										className="ml-1"
										onClick={() =>
											setAddEmails(addEmails.filter((e) => e !== email))
										}
									>
										×
									</button>
								</Badge>
							))}
						</div>
					</div>
				)}

				{canEdit && (
					<div className="flex flex-col gap-1">
						<div className="flex gap-2">
							<Input
								value={draft}
								onChange={(e) => {
									setDraft(e.target.value);
									if (draftError) setDraftError(null);
								}}
								onKeyDown={(e) => {
									if (e.key === "Enter" || e.key === ",") {
										e.preventDefault();
										stageDraftAdd();
									}
								}}
								placeholder="Add member email..."
								aria-label="Add member email"
								className="h-8 text-sm"
							/>
							<Button
								type="button"
								size="sm"
								variant="outline"
								onClick={stageDraftAdd}
							>
								Add
							</Button>
						</div>
						{draftError && (
							<p className="text-xs text-destructive">{draftError}</p>
						)}
					</div>
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

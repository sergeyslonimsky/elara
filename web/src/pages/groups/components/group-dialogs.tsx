import { useMutation, useQuery } from "@connectrpc/connect-query";
import { zodResolver } from "@hookform/resolvers/zod";
import { useQueryClient } from "@tanstack/react-query";
import { useEffect, useState } from "react";
import { useForm } from "react-hook-form";
import { toast } from "sonner";
import { z } from "zod";
import { canManageGroup } from "@/auth/ability";
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
import { Field, FieldLabel } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import type { Group } from "@/gen/elara/group/v1/group_pb";
import {
	createGroup,
	deleteGroup,
	listGroups,
} from "@/gen/elara/group/v1/group_service-GroupService_connectquery";
import { invalidate } from "@/lib/queries";
import { toastError } from "@/lib/toast";

const emailSchema = z.string().email();

const createSchema = z.object({
	name: z.string().min(1, "Name is required").max(128, "Max 128 characters"),
	description: z.string().max(1024, "Max 1024 characters"),
	initialMembers: z.array(z.string().email()),
	initialManagerGroupIds: z.array(z.string()),
});

type CreateFormValues = z.infer<typeof createSchema>;

interface CreateGroupDialogProps {
	open: boolean;
	onOpenChange: (open: boolean) => void;
}

export function CreateGroupDialog({
	open,
	onOpenChange,
}: Readonly<CreateGroupDialogProps>) {
	const queryClient = useQueryClient();
	const { state } = useAuth();
	const ability = state.status === "authenticated" ? state.ability : null;

	const [memberInput, setMemberInput] = useState("");
	const [memberError, setMemberError] = useState<string | null>(null);

	const form = useForm<CreateFormValues>({
		resolver: zodResolver(createSchema),
		defaultValues: {
			name: "",
			description: "",
			initialMembers: [],
			initialManagerGroupIds: [],
		},
		mode: "onChange",
	});

	useEffect(() => {
		if (!open) {
			form.reset({
				name: "",
				description: "",
				initialMembers: [],
				initialManagerGroupIds: [],
			});
			setMemberInput("");
			setMemberError(null);
		}
	}, [open, form]);

	const { data: groupsData } = useQuery(
		listGroups,
		{ pagination: { limit: 1000, offset: 0 } },
		{ enabled: open },
	);
	const manageableGroups = (groupsData?.groups ?? []).filter((g) =>
		ability ? canManageGroup(ability, g) : false,
	);

	const mutation = useMutation(createGroup, {
		onSuccess: (_data, vars) => {
			toast.success(`Group "${vars.name}" created`);
			onOpenChange(false);
			invalidate(queryClient, "groups");
		},
		onError: toastError,
	});

	const initialMembers = form.watch("initialMembers");
	const initialManagerGroupIds = form.watch("initialManagerGroupIds");

	const addMember = () => {
		const val = memberInput.trim();
		if (!val) {
			setMemberError(null);
			return;
		}
		const parsed = emailSchema.safeParse(val);
		if (!parsed.success) {
			setMemberError("Invalid email");
			return;
		}
		if (initialMembers.includes(val)) {
			setMemberError("Already added");
			return;
		}
		form.setValue("initialMembers", [...initialMembers, val], {
			shouldDirty: true,
			shouldValidate: true,
		});
		setMemberInput("");
		setMemberError(null);
	};

	const removeMember = (email: string) => {
		form.setValue(
			"initialMembers",
			initialMembers.filter((m) => m !== email),
			{ shouldDirty: true, shouldValidate: true },
		);
	};

	const toggleManagerGroup = (id: string) => {
		const next = new Set(initialManagerGroupIds);
		if (next.has(id)) next.delete(id);
		else next.add(id);
		form.setValue("initialManagerGroupIds", Array.from(next), {
			shouldDirty: true,
			shouldValidate: true,
		});
	};

	const onSubmit = (values: CreateFormValues) => {
		mutation.mutate({
			name: values.name.trim(),
			description: values.description.trim(),
			initialMembers: values.initialMembers,
			initialManagerGroupIds: values.initialManagerGroupIds,
		});
	};

	return (
		<Dialog open={open} onOpenChange={onOpenChange}>
			<DialogContent>
				<form onSubmit={form.handleSubmit(onSubmit)}>
					<DialogHeader>
						<DialogTitle>Create Group</DialogTitle>
						<DialogDescription>
							Groups bundle permissions for multiple users. Permissions can be
							added after creating the group.
						</DialogDescription>
					</DialogHeader>
					<div className="grid gap-4 py-4 max-h-[60vh] overflow-y-auto pr-1">
						<Field>
							<FieldLabel htmlFor="cg-name">Name</FieldLabel>
							<Input
								id="cg-name"
								placeholder="developers"
								{...form.register("name")}
							/>
							{form.formState.errors.name && (
								<p className="text-xs text-destructive">
									{form.formState.errors.name.message}
								</p>
							)}
						</Field>
						<Field>
							<FieldLabel htmlFor="cg-description">
								Description (optional)
							</FieldLabel>
							<Textarea
								id="cg-description"
								placeholder="Describe the purpose of this group..."
								maxLength={1024}
								{...form.register("description")}
							/>
							{form.formState.errors.description && (
								<p className="text-xs text-destructive">
									{form.formState.errors.description.message}
								</p>
							)}
						</Field>
						<Field>
							<FieldLabel>Initial members (optional)</FieldLabel>
							<div className="flex gap-2">
								<Input
									value={memberInput}
									onChange={(e) => {
										setMemberInput(e.target.value);
										if (memberError) setMemberError(null);
									}}
									onKeyDown={(e) => {
										if (e.key === "Enter" || e.key === ",") {
											e.preventDefault();
											addMember();
										}
									}}
									placeholder="user@example.com"
									aria-label="Add initial member"
									className="h-8 text-sm"
								/>
								<Button
									type="button"
									size="sm"
									variant="outline"
									onClick={addMember}
								>
									Add
								</Button>
							</div>
							{memberError && (
								<p className="text-xs text-destructive">{memberError}</p>
							)}
							{initialMembers.length > 0 && (
								<div className="flex flex-wrap gap-1 mt-1">
									{initialMembers.map((email) => (
										<Badge key={email} variant="secondary">
											{email}
											<button
												type="button"
												aria-label={`Remove ${email}`}
												onClick={() => removeMember(email)}
												className="ml-1 hover:text-destructive"
											>
												×
											</button>
										</Badge>
									))}
								</div>
							)}
						</Field>
						{manageableGroups.length > 0 && (
							<Field>
								<FieldLabel>Manager groups (optional)</FieldLabel>
								<p className="text-xs text-muted-foreground">
									Without manager groups, only superadmin can manage this group.
								</p>
								<div className="max-h-48 overflow-y-auto rounded-md border divide-y">
									{manageableGroups.map((g) => {
										const cbId = `cg-mgr-${g.id}`;
										const checked = initialManagerGroupIds.includes(g.id);
										return (
											<div
												key={g.id}
												className="flex items-center gap-2 p-2 text-sm"
											>
												<Checkbox
													id={cbId}
													checked={checked}
													onCheckedChange={() => toggleManagerGroup(g.id)}
													aria-label={g.name}
												/>
												<label htmlFor={cbId} className="cursor-pointer">
													{g.name}
												</label>
											</div>
										);
									})}
								</div>
							</Field>
						)}
					</div>
					<DialogFooter>
						<Button
							type="submit"
							disabled={!form.formState.isValid || mutation.isPending}
						>
							{mutation.isPending ? "Creating..." : "Create"}
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

function makeDeleteSchema(expected: string) {
	return z.object({
		confirm: z
			.string()
			.transform((v) => v.trim())
			.refine((v) => v === expected, "Group name does not match"),
	});
}

export function DeleteGroupDialog({
	group,
	open,
	onOpenChange,
}: Readonly<DeleteGroupDialogProps>) {
	const queryClient = useQueryClient();

	const schema = makeDeleteSchema(group?.name ?? "");
	type DeleteFormValues = z.infer<typeof schema>;

	const form = useForm<DeleteFormValues>({
		resolver: zodResolver(schema),
		defaultValues: { confirm: "" },
		mode: "onChange",
	});

	useEffect(() => {
		if (!open) form.reset({ confirm: "" });
	}, [open, form]);

	const mutation = useMutation(deleteGroup, {
		onSuccess: () => {
			toast.success("Group deleted");
			onOpenChange(false);
			invalidate(queryClient, "groups");
		},
		onError: toastError,
	});

	if (!group) return null;

	const onSubmit = () => {
		mutation.mutate({ id: group.id });
	};

	return (
		<Dialog open={open} onOpenChange={onOpenChange}>
			<DialogContent>
				<form onSubmit={form.handleSubmit(onSubmit)}>
					<DialogHeader>
						<DialogTitle>Delete Group</DialogTitle>
						<DialogDescription>
							This action is irreversible. Type <strong>{group.name}</strong> to
							confirm.
						</DialogDescription>
					</DialogHeader>
					<div className="py-4">
						<Input
							placeholder={group.name}
							aria-label="Confirm group name"
							{...form.register("confirm")}
						/>
					</div>
					<DialogFooter>
						<Button
							type="button"
							variant="outline"
							onClick={() => onOpenChange(false)}
						>
							Cancel
						</Button>
						<Button
							type="submit"
							variant="destructive"
							disabled={!form.formState.isValid || mutation.isPending}
						>
							{mutation.isPending ? "Deleting..." : "Delete group"}
						</Button>
					</DialogFooter>
				</form>
			</DialogContent>
		</Dialog>
	);
}

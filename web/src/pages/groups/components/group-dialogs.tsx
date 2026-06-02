import { useMutation, useQuery } from "@connectrpc/connect-query";
import { zodResolver } from "@hookform/resolvers/zod";
import { useQueryClient } from "@tanstack/react-query";
import { useEffect } from "react";
import { useForm } from "react-hook-form";
import { toast } from "sonner";
import { z } from "zod";
import { canManageGroup } from "@/auth/ability";
import { useAbility } from "@/auth/ability-context";
import { UserFilter } from "@/components/filters";
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

const createSchema = z.object({
	name: z.string().min(1, "Name is required").max(128, "Max 128 characters"),
	description: z.string().max(1024, "Max 1024 characters"),
	initialMembers: z.array(z.string().email()),
	initialManagerGroupNames: z.array(z.string()),
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
	const ability = useAbility();

	const form = useForm<CreateFormValues>({
		resolver: zodResolver(createSchema),
		defaultValues: {
			name: "",
			description: "",
			initialMembers: [],
			initialManagerGroupNames: [],
		},
		mode: "onChange",
	});

	useEffect(() => {
		if (!open) {
			form.reset({
				name: "",
				description: "",
				initialMembers: [],
				initialManagerGroupNames: [],
			});
		}
	}, [open, form]);

	const { data: groupsData } = useQuery(
		listGroups,
		{ pagination: { limit: 1000, offset: 0 } },
		{ enabled: open },
	);
	const manageableGroups = (groupsData?.groups ?? []).filter((g) =>
		canManageGroup(ability, g),
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
	const initialManagerGroupNames = form.watch("initialManagerGroupNames");

	const setMembers = (next: string[]) => {
		form.setValue("initialMembers", next, {
			shouldDirty: true,
			shouldValidate: true,
		});
	};

	const toggleManagerGroup = (name: string) => {
		const next = new Set(initialManagerGroupNames);
		if (next.has(name)) next.delete(name);
		else next.add(name);
		form.setValue("initialManagerGroupNames", Array.from(next), {
			shouldDirty: true,
			shouldValidate: true,
		});
	};

	const onSubmit = (values: CreateFormValues) => {
		mutation.mutate({
			name: values.name.trim(),
			description: values.description.trim(),
			initialMembers: values.initialMembers,
			initialManagerGroupNames: values.initialManagerGroupNames,
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
							<UserFilter value={initialMembers} onValueChange={setMembers} />
						</Field>
						{manageableGroups.length > 0 && (
							<Field>
								<FieldLabel>Manager groups (optional)</FieldLabel>
								<p className="text-xs text-muted-foreground">
									Without manager groups, only superadmin can manage this group.
								</p>
								<div className="max-h-48 overflow-y-auto rounded-md border divide-y">
									{manageableGroups.map((g) => {
										const cbId = `cg-mgr-${g.name}`;
										const checked = initialManagerGroupNames.includes(g.name);
										return (
											<div
												key={g.name}
												className="flex items-center gap-2 p-2 text-sm"
											>
												<Checkbox
													id={cbId}
													checked={checked}
													onCheckedChange={() => toggleManagerGroup(g.name)}
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
		mutation.mutate({ name: group.name });
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

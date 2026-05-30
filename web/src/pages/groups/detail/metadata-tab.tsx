import { Code, ConnectError } from "@connectrpc/connect";
import { useMutation } from "@connectrpc/connect-query";
import { zodResolver } from "@hookform/resolvers/zod";
import { useQueryClient } from "@tanstack/react-query";
import { useEffect } from "react";
import { useForm } from "react-hook-form";
import { toast } from "sonner";
import { z } from "zod";
import { canManageGroup } from "@/auth/ability";
import { useAbility } from "@/auth/ability-context";
import { Button } from "@/components/ui/button";
import { Field, FieldLabel } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import type { Group } from "@/gen/elara/group/v1/group_pb";
import { updateGroup } from "@/gen/elara/group/v1/group_service-GroupService_connectquery";
import { invalidate } from "@/lib/queries";
import { toastError } from "@/lib/toast";

const schema = z.object({
	name: z.string().min(1, "Name is required").max(128, "Max 128 characters"),
	description: z.string().max(1024, "Max 1024 characters"),
});

type FormValues = z.infer<typeof schema>;

interface MetadataTabProps {
	group: Group;
}

export function MetadataTab({ group }: Readonly<MetadataTabProps>) {
	const ability = useAbility();
	const queryClient = useQueryClient();

	const canEdit = canManageGroup(ability, group);

	const form = useForm<FormValues>({
		resolver: zodResolver(schema),
		defaultValues: { name: group.name, description: group.description },
		mode: "onChange",
	});

	// Sync form when the server publishes a NEW metadata version (e.g. after
	// a successful save, or another tab edited it). Anchored on
	// metadataVersion so refetches that don't change the version don't blow
	// away in-progress edits.
	// biome-ignore lint/correctness/useExhaustiveDependencies: only re-sync on version change
	useEffect(() => {
		form.reset({ name: group.name, description: group.description });
	}, [group.metadataVersion, form]);

	const mutation = useMutation(updateGroup, {
		onSuccess: () => {
			toast.success("Metadata updated");
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

	const onSubmit = (values: FormValues) => {
		if (!canEdit) return;
		mutation.mutate({
			id: group.id,
			name: values.name.trim(),
			description: values.description.trim(),
			expectedMetadataVersion: group.metadataVersion,
		});
	};

	const nameError = form.formState.errors.name?.message;
	const descriptionError = form.formState.errors.description?.message;

	return (
		<div className="mt-2 rounded-xl border bg-card p-4">
			{group.isSystem && (
				<p className="mb-3 text-sm text-muted-foreground">
					This is a system group and cannot be modified.
				</p>
			)}
			<form
				onSubmit={form.handleSubmit(onSubmit)}
				className="flex flex-col gap-4"
			>
				<Field>
					<FieldLabel htmlFor="meta-name">Name</FieldLabel>
					<Input
						id="meta-name"
						disabled={!canEdit}
						{...form.register("name")}
					/>
					{nameError && <p className="text-xs text-destructive">{nameError}</p>}
				</Field>
				<Field>
					<FieldLabel htmlFor="meta-description">Description</FieldLabel>
					<Textarea
						id="meta-description"
						disabled={!canEdit}
						maxLength={1024}
						{...form.register("description")}
					/>
					{descriptionError && (
						<p className="text-xs text-destructive">{descriptionError}</p>
					)}
				</Field>
				<div className="flex justify-end">
					<Button
						type="submit"
						size="sm"
						disabled={
							!canEdit ||
							!form.formState.isDirty ||
							!form.formState.isValid ||
							mutation.isPending
						}
					>
						{mutation.isPending ? "Saving..." : "Save changes"}
					</Button>
				</div>
			</form>
		</div>
	);
}

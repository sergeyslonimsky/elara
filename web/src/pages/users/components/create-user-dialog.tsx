import { useMutation, useQuery } from "@connectrpc/connect-query";
import { zodResolver } from "@hookform/resolvers/zod";
import { useQueryClient } from "@tanstack/react-query";
import { Plus } from "lucide-react";
import { useEffect, useState } from "react";
import { useForm } from "react-hook-form";
import { toast } from "sonner";
import { z } from "zod";
import { canManageGroup } from "@/auth/ability";
import { useAbility } from "@/auth/ability-context";
import { useAuth } from "@/components/auth-provider";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import {
	Dialog,
	DialogContent,
	DialogDescription,
	DialogFooter,
	DialogHeader,
	DialogTitle,
	DialogTrigger,
} from "@/components/ui/dialog";
import { Field, FieldLabel } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { AuthType } from "@/gen/elara/auth/v1/auth_pb";
import { listGroups } from "@/gen/elara/group/v1/group_service-GroupService_connectquery";
import { createUser } from "@/gen/elara/user/v1/user_service-UserService_connectquery";
import { invalidate, invalidateUserGroupGraph } from "@/lib/queries";
import { toastError } from "@/lib/toast";

const MIN_PASSWORD_LENGTH = 8;

function makeSchema(isBasicAuth: boolean) {
	return z.object({
		email: z.string().email("Invalid email format"),
		name: z.string().min(1, "Name is required"),
		initialPassword: isBasicAuth
			? z
					.string()
					.min(
						MIN_PASSWORD_LENGTH,
						`Password must be at least ${MIN_PASSWORD_LENGTH} characters`,
					)
			: z.string().optional(),
		initialGroupIds: z.array(z.string()),
	});
}

export function CreateUserDialog() {
	const { state } = useAuth();
	const ability = useAbility();
	const [open, setOpen] = useState(false);
	const queryClient = useQueryClient();

	const canCreate = ability.can("create", "User");
	const authType =
		state.status === "authenticated" || state.status === "anonymous"
			? state.authType
			: AuthType.UNSPECIFIED;
	const isBasicAuth = authType === AuthType.BASIC;

	const schema = makeSchema(isBasicAuth);
	type FormValues = z.infer<typeof schema>;

	const form = useForm<FormValues>({
		resolver: zodResolver(schema),
		defaultValues: {
			email: "",
			name: "",
			initialPassword: "",
			initialGroupIds: [],
		},
		mode: "onChange",
	});

	useEffect(() => {
		if (!open) {
			form.reset({
				email: "",
				name: "",
				initialPassword: "",
				initialGroupIds: [],
			});
		}
	}, [open, form]);

	// Load groups so the caller can pre-assign memberships.
	const { data: groupsData, isLoading: groupsLoading } = useQuery(
		listGroups,
		{ pagination: { limit: 1000, offset: 0 } },
		{ enabled: open },
	);
	const manageableGroups = (groupsData?.groups ?? []).filter((g) =>
		canManageGroup(ability, g),
	);

	const mutation = useMutation(createUser, {
		onSuccess: (_data, vars) => {
			toast.success(`User "${vars.email}" created`);
			setOpen(false);
			if (vars.initialGroupIds && vars.initialGroupIds.length > 0) {
				invalidateUserGroupGraph(queryClient);
			} else {
				invalidate(queryClient, "users");
			}
		},
		onError: toastError,
	});

	if (!canCreate) {
		return null;
	}

	const onSubmit = (values: FormValues) => {
		mutation.mutate({
			email: values.email.trim(),
			name: values.name.trim(),
			initialPassword: isBasicAuth ? (values.initialPassword ?? "") : "",
			initialGroupIds: values.initialGroupIds,
		});
	};

	const selectedGroupIds = form.watch("initialGroupIds");

	const toggleGroup = (id: string) => {
		const current = new Set(selectedGroupIds);
		if (current.has(id)) current.delete(id);
		else current.add(id);
		form.setValue("initialGroupIds", Array.from(current), {
			shouldDirty: true,
			shouldValidate: true,
		});
	};

	return (
		<Dialog open={open} onOpenChange={setOpen}>
			<DialogTrigger render={<Button size="sm" />}>
				<Plus className="mr-1 h-4 w-4" />
				New User
			</DialogTrigger>
			<DialogContent>
				<form onSubmit={form.handleSubmit(onSubmit)}>
					<DialogHeader>
						<DialogTitle>Create User</DialogTitle>
						<DialogDescription>
							Create a user account. Group memberships can be assigned now or
							later from the user's detail page.
						</DialogDescription>
					</DialogHeader>
					<div className="grid gap-4 py-4">
						<Field>
							<FieldLabel htmlFor="cu-email">Email</FieldLabel>
							<Input
								id="cu-email"
								type="email"
								placeholder="user@example.com"
								{...form.register("email")}
							/>
							{form.formState.errors.email && (
								<p className="text-xs text-destructive">
									{form.formState.errors.email.message}
								</p>
							)}
						</Field>
						<Field>
							<FieldLabel htmlFor="cu-name">Name</FieldLabel>
							<Input
								id="cu-name"
								placeholder="Jane Doe"
								{...form.register("name")}
							/>
							{form.formState.errors.name && (
								<p className="text-xs text-destructive">
									{form.formState.errors.name.message}
								</p>
							)}
						</Field>
						{isBasicAuth && (
							<Field>
								<FieldLabel htmlFor="cu-password">Initial password</FieldLabel>
								<Input
									id="cu-password"
									type="password"
									placeholder={`At least ${MIN_PASSWORD_LENGTH} characters`}
									{...form.register("initialPassword")}
								/>
								{form.formState.errors.initialPassword && (
									<p className="text-xs text-destructive">
										{form.formState.errors.initialPassword.message}
									</p>
								)}
							</Field>
						)}
						{groupsLoading ? (
							<Field>
								<FieldLabel>Initial groups (optional)</FieldLabel>
								<p className="text-xs text-muted-foreground">Loading groups…</p>
							</Field>
						) : (
							manageableGroups.length > 0 && (
								<Field>
									<FieldLabel>Initial groups (optional)</FieldLabel>
									<div className="max-h-48 overflow-y-auto rounded-md border divide-y">
										{manageableGroups.map((g) => {
											const cbId = `cu-group-${g.id}`;
											const checked = selectedGroupIds.includes(g.id);
											return (
												<div
													key={g.id}
													className="flex items-center gap-2 p-2 text-sm"
												>
													<Checkbox
														id={cbId}
														checked={checked}
														onCheckedChange={() => toggleGroup(g.id)}
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
							)
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

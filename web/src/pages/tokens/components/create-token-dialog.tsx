import { create } from "@bufbuild/protobuf";
import { TimestampSchema } from "@bufbuild/protobuf/wkt";
import { useMutation } from "@connectrpc/connect-query";
import { zodResolver } from "@hookform/resolvers/zod";
import { useQueryClient } from "@tanstack/react-query";
import { Plus } from "lucide-react";
import { useEffect, useState } from "react";
import { useForm } from "react-hook-form";
import { toast } from "sonner";
import { z } from "zod";
import { Button } from "@/components/ui/button";
import {
	Dialog,
	DialogContent,
	DialogDescription,
	DialogFooter,
	DialogHeader,
	DialogTitle,
	DialogTrigger,
} from "@/components/ui/dialog";
import { Field, FieldDescription, FieldLabel } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import {
	Select,
	SelectContent,
	SelectItem,
	SelectTrigger,
	SelectValue,
} from "@/components/ui/select";
import { PermissionAction } from "@/gen/elara/common/v1/permission_pb";
import { createToken } from "@/gen/elara/token/v1/token_service-TokenService_connectquery";
import { invalidate } from "@/lib/queries";
import { toastError } from "@/lib/toast";
import { NamespaceMultiSelect } from "./namespace-multi-select";
import { TokenCreatedView } from "./token-created-view";

const PermissionValues = ["read", "write"] as const;

// Mirrors the server-side rule for the username prefix.
export const NAME_PREFIX_PATTERN = /^[a-z0-9]([a-z0-9._-]*[a-z0-9])?$/;
export const NAME_PREFIX_MAX_LENGTH = 48;

const schema = z.object({
	name: z
		.string()
		.trim()
		.min(1, "Name prefix is required")
		.max(
			NAME_PREFIX_MAX_LENGTH,
			`Name prefix must be at most ${NAME_PREFIX_MAX_LENGTH} characters`,
		)
		.regex(
			NAME_PREFIX_PATTERN,
			"Use only lowercase letters, digits, '.', '_' and '-', starting and ending with a letter or digit",
		),
	namespaces: z.array(z.string()).min(1, "Select at least one namespace"),
	permission: z.enum(PermissionValues),
	expiresAt: z.string().optional(),
});

type FormValues = z.infer<typeof schema>;

function permissionStringToProto(p: "read" | "write"): PermissionAction {
	return p === "read" ? PermissionAction.READ : PermissionAction.WRITE;
}

function toTimestamp(value: string | undefined) {
	if (!value) return undefined;
	const ms = new Date(value).getTime();
	if (Number.isNaN(ms)) return undefined;
	const seconds = BigInt(Math.floor(ms / 1000));
	const nanos = (ms % 1000) * 1_000_000;
	return create(TimestampSchema, { seconds, nanos });
}

interface CreatedToken {
	readonly username: string;
	readonly rawToken: string;
}

export function CreateTokenDialog() {
	const [open, setOpen] = useState(false);
	const [created, setCreated] = useState<CreatedToken | null>(null);
	const queryClient = useQueryClient();

	const form = useForm<FormValues>({
		resolver: zodResolver(schema),
		defaultValues: {
			name: "",
			namespaces: [],
			permission: "read",
			expiresAt: "",
		},
		mode: "onChange",
	});

	useEffect(() => {
		if (!open) {
			form.reset({
				name: "",
				namespaces: [],
				permission: "read",
				expiresAt: "",
			});
			setCreated(null);
		}
	}, [open, form]);

	const mutation = useMutation(createToken, {
		onSuccess: (data, vars) => {
			const username = data.token?.username || vars.name || "";
			toast.success(`Token "${username}" created`);
			invalidate(queryClient, "tokens");
			setCreated({ username, rawToken: data.rawToken });
		},
		onError: toastError,
	});

	const onSubmit = (values: FormValues) => {
		mutation.mutate({
			name: values.name.trim(),
			namespaces: values.namespaces,
			permission: permissionStringToProto(values.permission),
			expiresAt: toTimestamp(values.expiresAt),
		});
	};

	const namespaces = form.watch("namespaces");
	const nameValue = form.watch("name").trim();
	const namePreview =
		nameValue && !form.formState.errors.name ? `${nameValue}-xxxxxx` : null;
	const permission = form.watch("permission");

	return (
		<Dialog open={open} onOpenChange={setOpen}>
			<DialogTrigger render={<Button size="sm" />}>
				<Plus className="mr-1 h-4 w-4" />
				New Token
			</DialogTrigger>
			<DialogContent>
				{created ? (
					<TokenCreatedView
						username={created.username}
						rawToken={created.rawToken}
						onDone={() => setOpen(false)}
					/>
				) : (
					<form onSubmit={form.handleSubmit(onSubmit)}>
						<DialogHeader>
							<DialogTitle>Create Token</DialogTitle>
							<DialogDescription>
								Create an API token scoped to one or more namespaces.
							</DialogDescription>
						</DialogHeader>
						<div className="grid gap-4 py-4">
							<Field>
								<FieldLabel htmlFor="ct-name">Name prefix</FieldLabel>
								<Input
									id="ct-name"
									placeholder="ci-deployer"
									aria-describedby="ct-name-help"
									aria-invalid={Boolean(form.formState.errors.name)}
									{...form.register("name")}
								/>
								<FieldDescription id="ct-name-help" className="text-xs">
									The server appends a random suffix to make the username
									unique, e.g. <code>ci</code> → <code>ci-k7x2pq</code>.
									Lowercase letters, digits, '.', '_' and '-'.
								</FieldDescription>
								{namePreview && (
									<p className="text-muted-foreground text-xs">
										Username:{" "}
										<code className="font-mono text-foreground">
											{namePreview}
										</code>
									</p>
								)}
								{form.formState.errors.name && (
									<p role="alert" className="text-xs text-destructive">
										{form.formState.errors.name.message}
									</p>
								)}
							</Field>

							<Field>
								<FieldLabel htmlFor="ct-namespaces">Namespaces</FieldLabel>
								<NamespaceMultiSelect
									id="ct-namespaces"
									value={namespaces}
									onChange={(next) =>
										form.setValue("namespaces", next, {
											shouldDirty: true,
											shouldValidate: true,
										})
									}
								/>
								{form.formState.errors.namespaces && (
									<p className="text-xs text-destructive">
										{form.formState.errors.namespaces.message}
									</p>
								)}
							</Field>

							<Field>
								<FieldLabel htmlFor="ct-permission">Permission</FieldLabel>
								<Select
									value={permission}
									onValueChange={(v) => {
										if (v === "read" || v === "write") {
											form.setValue("permission", v, {
												shouldDirty: true,
												shouldValidate: true,
											});
										}
									}}
								>
									<SelectTrigger id="ct-permission" className="w-full">
										<SelectValue />
									</SelectTrigger>
									<SelectContent>
										<SelectItem value="read">Read</SelectItem>
										<SelectItem value="write">Write</SelectItem>
									</SelectContent>
								</Select>
							</Field>

							<Field>
								<FieldLabel htmlFor="ct-expires">
									Expires at (optional)
								</FieldLabel>
								<Input
									id="ct-expires"
									type="datetime-local"
									{...form.register("expiresAt")}
								/>
							</Field>
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
				)}
			</DialogContent>
		</Dialog>
	);
}

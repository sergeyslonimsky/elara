import { create } from "@bufbuild/protobuf";
import { TimestampSchema } from "@bufbuild/protobuf/wkt";
import { useMutation } from "@connectrpc/connect-query";
import { zodResolver } from "@hookform/resolvers/zod";
import { useQueryClient } from "@tanstack/react-query";
import { Check, Copy, Plus } from "lucide-react";
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
import { Field, FieldLabel } from "@/components/ui/field";
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

const PermissionValues = ["read", "write"] as const;

const schema = z.object({
	name: z.string().trim().min(1, "Name is required"),
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

export function CreateTokenDialog() {
	const [open, setOpen] = useState(false);
	const [rawToken, setRawToken] = useState<string | null>(null);
	const [copied, setCopied] = useState(false);
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
			setRawToken(null);
			setCopied(false);
		}
	}, [open, form]);

	const mutation = useMutation(createToken, {
		onSuccess: (data, vars) => {
			toast.success(`Token "${vars.name}" created`);
			invalidate(queryClient, "tokens");
			setRawToken(data.rawToken);
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

	const handleCopy = async () => {
		if (!rawToken) return;
		try {
			await navigator.clipboard.writeText(rawToken);
			setCopied(true);
			setTimeout(() => setCopied(false), 2000);
		} catch {
			toast.error("Failed to copy token");
		}
	};

	const namespaces = form.watch("namespaces");
	const permission = form.watch("permission");

	return (
		<Dialog open={open} onOpenChange={setOpen}>
			<DialogTrigger render={<Button size="sm" />}>
				<Plus className="mr-1 h-4 w-4" />
				New Token
			</DialogTrigger>
			<DialogContent>
				{rawToken ? (
					<>
						<DialogHeader>
							<DialogTitle>Token created</DialogTitle>
							<DialogDescription>
								Copy this token now — it will not be shown again.
							</DialogDescription>
						</DialogHeader>
						<div className="flex flex-col gap-2 py-2">
							<div className="flex items-stretch gap-2">
								<Input
									readOnly
									value={rawToken}
									className="font-mono text-xs"
									onFocus={(e) => e.currentTarget.select()}
									aria-label="Raw token value"
								/>
								<Button
									type="button"
									variant="outline"
									size="icon"
									onClick={handleCopy}
									aria-label="Copy token"
								>
									{copied ? (
										<Check className="h-4 w-4 text-emerald-500" />
									) : (
										<Copy className="h-4 w-4" />
									)}
								</Button>
							</div>
							<p className="text-muted-foreground text-xs">
								Store it in a secret manager — this is the only time the raw
								token is displayed.
							</p>
						</div>
						<DialogFooter>
							<Button onClick={() => setOpen(false)}>Done</Button>
						</DialogFooter>
					</>
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
								<FieldLabel htmlFor="ct-name">Name</FieldLabel>
								<Input
									id="ct-name"
									placeholder="ci-deployer"
									{...form.register("name")}
								/>
								{form.formState.errors.name && (
									<p className="text-xs text-destructive">
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

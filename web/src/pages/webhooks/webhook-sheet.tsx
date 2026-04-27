import { useMutation } from "@connectrpc/connect-query";
import { useQueryClient } from "@tanstack/react-query";
import { useEffect, useState } from "react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Field, FieldLabel } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import {
	Sheet,
	SheetContent,
	SheetFooter,
	SheetHeader,
	SheetTitle,
} from "@/components/ui/sheet";
import type { Webhook } from "@/gen/elara/webhook/v1/webhook_pb";
import { WebhookEvent } from "@/gen/elara/webhook/v1/webhook_pb";
import {
	createWebhook,
	updateWebhook,
} from "@/gen/elara/webhook/v1/webhook_service-WebhookService_connectquery";
import { invalidate } from "@/lib/queries";
import { toastError } from "@/lib/toast";

const ALL_EVENTS = [
	{ value: WebhookEvent.CREATED, label: "Created" },
	{ value: WebhookEvent.UPDATED, label: "Updated" },
	{ value: WebhookEvent.DELETED, label: "Deleted" },
] as const;

interface FormState {
	url: string;
	namespaceFilter: string;
	pathPrefix: string;
	events: WebhookEvent[];
	secret: string;
	enabled: boolean;
}

function defaultForm(): FormState {
	return {
		url: "",
		namespaceFilter: "",
		pathPrefix: "",
		events: [WebhookEvent.CREATED, WebhookEvent.UPDATED, WebhookEvent.DELETED],
		secret: "",
		enabled: true,
	};
}

interface WebhookSheetProps {
	open: boolean;
	onOpenChange: (open: boolean) => void;
	webhook?: Webhook;
}

export function WebhookSheet({
	open,
	onOpenChange,
	webhook,
}: WebhookSheetProps) {
	const isEdit = !!webhook;
	const queryClient = useQueryClient();
	const [form, setForm] = useState<FormState>(defaultForm());

	useEffect(() => {
		if (!open) return;
		if (webhook) {
			setForm({
				url: webhook.url,
				namespaceFilter: webhook.namespaceFilter,
				pathPrefix: webhook.pathPrefix,
				events: [...webhook.events],
				secret: "",
				enabled: webhook.enabled,
			});
		} else {
			setForm(defaultForm());
		}
	}, [open, webhook]);

	const createMutation = useMutation(createWebhook, {
		onSuccess: () => {
			toast.success("Webhook created");
			onOpenChange(false);
			invalidate(queryClient, "webhooks");
		},
		onError: toastError,
	});

	const updateMutation = useMutation(updateWebhook, {
		onSuccess: () => {
			toast.success("Webhook saved");
			onOpenChange(false);
			invalidate(queryClient, "webhooks");
		},
		onError: toastError,
	});

	const isPending = createMutation.isPending || updateMutation.isPending;
	const canSubmit = form.url.trim() !== "" && form.events.length > 0;

	function toggleEvent(event: WebhookEvent) {
		setForm((f) => ({
			...f,
			events: f.events.includes(event)
				? f.events.filter((e) => e !== event)
				: [...f.events, event],
		}));
	}

	function handleSubmit(e: React.FormEvent) {
		e.preventDefault();
		if (isEdit && webhook) {
			updateMutation.mutate({ id: webhook.id, ...form });
		} else {
			createMutation.mutate(form);
		}
	}

	return (
		<Sheet open={open} onOpenChange={onOpenChange}>
			<SheetContent className="sm:max-w-md overflow-y-auto">
				<form onSubmit={handleSubmit} className="flex flex-col h-full">
					<SheetHeader>
						<SheetTitle>{isEdit ? "Edit Webhook" : "Add Webhook"}</SheetTitle>
					</SheetHeader>

					<div className="flex flex-col gap-4 flex-1 px-4 py-2">
						<Field>
							<FieldLabel>URL</FieldLabel>
							<Input
								value={form.url}
								onChange={(e) =>
									setForm((f) => ({ ...f, url: e.target.value }))
								}
								placeholder="https://example.com/webhook"
								required
							/>
						</Field>

						<Field>
							<FieldLabel>Events</FieldLabel>
							<div className="flex gap-4 pt-1">
								{ALL_EVENTS.map(({ value, label }) => (
									<button
										key={value}
										type="button"
										className="flex items-center gap-2 text-sm cursor-pointer select-none"
										onClick={() => toggleEvent(value)}
									>
										<Checkbox
											checked={form.events.includes(value)}
											onCheckedChange={() => toggleEvent(value)}
										/>
										{label}
									</button>
								))}
							</div>
						</Field>

						<Field>
							<FieldLabel>Namespace filter</FieldLabel>
							<Input
								value={form.namespaceFilter}
								onChange={(e) =>
									setForm((f) => ({ ...f, namespaceFilter: e.target.value }))
								}
								placeholder="production (empty = all namespaces)"
							/>
						</Field>

						<Field>
							<FieldLabel>Path prefix</FieldLabel>
							<Input
								value={form.pathPrefix}
								onChange={(e) =>
									setForm((f) => ({ ...f, pathPrefix: e.target.value }))
								}
								placeholder="/services/ (empty = all paths)"
							/>
						</Field>

						<Field>
							<FieldLabel>Secret</FieldLabel>
							<Input
								value={form.secret}
								onChange={(e) =>
									setForm((f) => ({ ...f, secret: e.target.value }))
								}
								placeholder={
									isEdit
										? "Leave blank to keep existing secret"
										: "HMAC-SHA256 signing secret (optional)"
								}
								type="password"
								autoComplete="new-password"
							/>
						</Field>

						<button
							type="button"
							className="flex items-center gap-2 text-sm cursor-pointer select-none"
							onClick={() => setForm((f) => ({ ...f, enabled: !f.enabled }))}
						>
							<Checkbox
								checked={form.enabled}
								onCheckedChange={(checked) =>
									setForm((f) => ({ ...f, enabled: !!checked }))
								}
							/>
							Enabled
						</button>
					</div>

					<SheetFooter>
						<Button
							type="submit"
							disabled={isPending || !canSubmit}
							className="w-full"
						>
							{isPending
								? isEdit
									? "Saving..."
									: "Creating..."
								: isEdit
									? "Save changes"
									: "Create webhook"}
						</Button>
					</SheetFooter>
				</form>
			</SheetContent>
		</Sheet>
	);
}

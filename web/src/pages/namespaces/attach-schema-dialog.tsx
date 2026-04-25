import { useMutation } from "@connectrpc/connect-query";
import { useQueryClient } from "@tanstack/react-query";
import { Pencil, Plus, Sparkles } from "lucide-react";
import { useState } from "react";
import { toast } from "sonner";
import { ConfigEditor } from "@/components/config-editor";
import { Button } from "@/components/ui/button";
import {
	Dialog,
	DialogContent,
	DialogFooter,
	DialogHeader,
	DialogTitle,
	DialogTrigger,
} from "@/components/ui/dialog";
import { Field, FieldLabel } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { attachSchema } from "@/gen/elara/config/v1/schema_service-SchemaService_connectquery";
import { invalidate } from "@/lib/queries";
import { toastError } from "@/lib/toast";

interface AttachSchemaDialogProps {
	namespace: string;
	initialPathPattern?: string;
	initialJsonSchema?: string;
	disabled?: boolean;
}

export function AttachSchemaDialog({
	namespace,
	initialPathPattern,
	initialJsonSchema,
	disabled,
}: Readonly<AttachSchemaDialogProps>) {
	const isEdit = !!initialPathPattern;
	const queryClient = useQueryClient();
	const [open, setOpen] = useState(false);
	const [pathPattern, setPathPattern] = useState(initialPathPattern ?? "");
	const [jsonSchema, setJsonSchema] = useState(
		initialJsonSchema ?? '{\n  "type": "object"\n}',
	);

	function handleOpenChange(next: boolean) {
		setOpen(next);
		if (next) {
			setPathPattern(initialPathPattern ?? "");
			setJsonSchema(initialJsonSchema ?? '{\n  "type": "object"\n}');
		}
	}

	const mutation = useMutation(attachSchema, {
		onSuccess: () => {
			toast.success(isEdit ? "Schema updated" : "Schema attached");
			invalidate(queryClient, "schemas");
			setOpen(false);
		},
		onError: toastError,
	});

	function formatSchema() {
		try {
			setJsonSchema(JSON.stringify(JSON.parse(jsonSchema), null, 2));
		} catch {
			toast.error("Invalid JSON — cannot format");
		}
	}

	return (
		<Dialog open={open} onOpenChange={handleOpenChange}>
			<DialogTrigger
				render={
					<Button
						size={isEdit ? "icon-xs" : "sm"}
						variant={isEdit ? "ghost" : "default"}
						disabled={disabled}
					/>
				}
			>
				{isEdit ? (
					<Pencil className="h-3.5 w-3.5" />
				) : (
					<>
						<Plus className="mr-1 h-3.5 w-3.5" />
						Attach Schema
					</>
				)}
			</DialogTrigger>
			<DialogContent className="sm:max-w-3xl">
				<form
					onSubmit={(e) => {
						e.preventDefault();
						mutation.mutate({ namespace, pathPattern, jsonSchema });
					}}
				>
					<DialogHeader>
						<DialogTitle>
							{isEdit ? "Edit JSON Schema" : "Attach JSON Schema"}
						</DialogTitle>
					</DialogHeader>
					<div className="grid gap-4 py-4">
						<Field>
							<FieldLabel>Path Pattern</FieldLabel>
							<Input
								placeholder="/configs/**"
								value={pathPattern}
								onChange={(e) => setPathPattern(e.target.value)}
								required
								disabled={isEdit}
							/>
						</Field>
						<Field>
							<div className="flex items-center justify-between">
								<FieldLabel>JSON Schema</FieldLabel>
								<Button
									type="button"
									variant="outline"
									size="xs"
									onClick={formatSchema}
								>
									<Sparkles className="mr-1 h-3 w-3" />
									Format
								</Button>
							</div>
							<ConfigEditor
								value={jsonSchema}
								onChange={(v) => setJsonSchema(v ?? "")}
								language="json"
								height="350px"
							/>
						</Field>
					</div>
					<DialogFooter>
						<Button
							variant="outline"
							type="button"
							onClick={() => setOpen(false)}
						>
							Cancel
						</Button>
						<Button
							type="submit"
							disabled={
								mutation.isPending || !pathPattern.trim() || !jsonSchema.trim()
							}
						>
							{mutation.isPending
								? isEdit
									? "Saving..."
									: "Attaching..."
								: isEdit
									? "Save"
									: "Attach"}
						</Button>
					</DialogFooter>
				</form>
			</DialogContent>
		</Dialog>
	);
}

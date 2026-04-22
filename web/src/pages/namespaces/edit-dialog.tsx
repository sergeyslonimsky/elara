import { useMutation } from "@connectrpc/connect-query";
import { useQueryClient } from "@tanstack/react-query";
import { Pencil } from "lucide-react";
import { useState } from "react";
import { toast } from "sonner";
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
import { Textarea } from "@/components/ui/textarea";
import { updateNamespace } from "@/gen/elara/namespace/v1/namespace_service-NamespaceService_connectquery";
import { invalidate } from "@/lib/queries";
import { toastError } from "@/lib/toast";

export function EditDialog({
	name,
	currentDescription,
	locked,
}: Readonly<{
	name: string;
	currentDescription: string;
	locked: boolean;
}>) {
	const [open, setOpen] = useState(false);
	const [description, setDescription] = useState(currentDescription);
	const queryClient = useQueryClient();

	const handleOpenChange = (isOpen: boolean) => {
		if (isOpen) setDescription(currentDescription);
		setOpen(isOpen);
	};

	const mutation = useMutation(updateNamespace, {
		onSuccess: () => {
			toast.success(`Namespace "${name}" updated`);
			setOpen(false);
			invalidate(queryClient, "namespaces");
		},
		onError: toastError,
	});

	return (
		<Dialog open={open} onOpenChange={handleOpenChange}>
			<DialogTrigger
				render={
					<Button
						variant="ghost"
						size="icon-xs"
						disabled={locked ? true : undefined}
						title={locked ? `Namespace "${name}" is locked` : undefined}
					/>
				}
			>
				<Pencil className="h-3.5 w-3.5" />
			</DialogTrigger>
			<DialogContent>
				<form
					onSubmit={(e) => {
						e.preventDefault();
						mutation.mutate({ name, description });
					}}
				>
					<DialogHeader>
						<DialogTitle>Edit "{name}"</DialogTitle>
					</DialogHeader>
					<div className="grid gap-4 py-4">
						<Field>
							<FieldLabel>Description</FieldLabel>
							<Textarea
								value={description}
								onChange={(e) => setDescription(e.target.value)}
							/>
						</Field>
					</div>
					<DialogFooter>
						<Button
							variant="outline"
							type="button"
							onClick={() => setOpen(false)}
						>
							Discard
						</Button>
						<Button type="submit" disabled={mutation.isPending}>
							{mutation.isPending ? "Saving..." : "Save"}
						</Button>
					</DialogFooter>
				</form>
			</DialogContent>
		</Dialog>
	);
}

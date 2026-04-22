import { useMutation } from "@connectrpc/connect-query";
import { useQueryClient } from "@tanstack/react-query";
import { Plus } from "lucide-react";
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
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { createNamespace } from "@/gen/elara/namespace/v1/namespace_service-NamespaceService_connectquery";
import { invalidate } from "@/lib/queries";
import { toastError } from "@/lib/toast";

export function CreateDialog() {
	const [open, setOpen] = useState(false);
	const [name, setName] = useState("");
	const [description, setDescription] = useState("");
	const queryClient = useQueryClient();

	const mutation = useMutation(createNamespace, {
		onSuccess: () => {
			toast.success(`Namespace "${name}" created`);
			setOpen(false);
			setName("");
			setDescription("");
			invalidate(queryClient, "namespaces");
		},
		onError: toastError,
	});

	return (
		<Dialog open={open} onOpenChange={setOpen}>
			<DialogTrigger render={<Button size="sm" />}>
				<Plus className="mr-1 h-4 w-4" />
				New Namespace
			</DialogTrigger>
			<DialogContent>
				<form
					onSubmit={(e) => {
						e.preventDefault();
						mutation.mutate({ name, description });
					}}
				>
					<DialogHeader>
						<DialogTitle>Create Namespace</DialogTitle>
					</DialogHeader>
					<div className="grid gap-4 py-4">
						<Field>
							<FieldLabel>Name</FieldLabel>
							<Input
								value={name}
								onChange={(e) => setName(e.target.value)}
								placeholder="production"
								required
							/>
						</Field>
						<Field>
							<FieldLabel>Description</FieldLabel>
							<Textarea
								value={description}
								onChange={(e) => setDescription(e.target.value)}
								placeholder="Production environment configs"
							/>
						</Field>
					</div>
					<DialogFooter>
						<Button type="submit" disabled={mutation.isPending || !name}>
							{mutation.isPending ? "Creating..." : "Create"}
						</Button>
					</DialogFooter>
				</form>
			</DialogContent>
		</Dialog>
	);
}

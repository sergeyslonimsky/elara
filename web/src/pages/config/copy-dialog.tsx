import { useMutation } from "@connectrpc/connect-query";
import { useQueryClient } from "@tanstack/react-query";
import { Copy } from "lucide-react";
import { useState } from "react";
import { toast } from "sonner";
import { NamespaceSelect } from "@/components/namespace-select";
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
import { copyConfig } from "@/gen/elara/config/v1/config_service-ConfigService_connectquery";
import { invalidate } from "@/lib/queries";
import { toastError } from "@/lib/toast";

interface CopyDialogProps {
	sourcePath: string;
	sourceNamespace: string;
}

export function CopyDialog({ sourcePath, sourceNamespace }: CopyDialogProps) {
	const [open, setOpen] = useState(false);
	const [destPath, setDestPath] = useState(sourcePath);
	const [destNamespace, setDestNamespace] = useState(sourceNamespace);
	const queryClient = useQueryClient();

	const mutation = useMutation(copyConfig, {
		onSuccess: () => {
			toast.success(`Config copied to ${destPath}`);
			setOpen(false);
			void invalidate(queryClient, "configs");
		},
		onError: toastError,
	});

	return (
		<Dialog open={open} onOpenChange={setOpen}>
			<DialogTrigger render={<Button variant="outline" size="sm" />}>
				<Copy className="mr-1 h-4 w-4" />
				Copy
			</DialogTrigger>
			<DialogContent>
				<form
					onSubmit={(e) => {
						e.preventDefault();
						mutation.mutate({
							sourcePath,
							sourceNamespace,
							destinationPath: destPath,
							destinationNamespace: destNamespace,
						});
					}}
				>
					<DialogHeader>
						<DialogTitle>Copy Config</DialogTitle>
					</DialogHeader>
					<div className="grid gap-4 py-4">
						<Field>
							<FieldLabel>Destination Path</FieldLabel>
							<Input
								value={destPath}
								onChange={(e) => setDestPath(e.target.value)}
								required
							/>
						</Field>
						<Field>
							<FieldLabel>Destination Namespace</FieldLabel>
							<NamespaceSelect
								value={destNamespace}
								onChange={setDestNamespace}
							/>
						</Field>
					</div>
					<DialogFooter>
						<Button type="submit" disabled={mutation.isPending}>
							{mutation.isPending ? "Copying..." : "Copy"}
						</Button>
					</DialogFooter>
				</form>
			</DialogContent>
		</Dialog>
	);
}

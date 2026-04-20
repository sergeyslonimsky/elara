import { useMutation } from "@connectrpc/connect-query";
import { Upload } from "lucide-react";
import { useRef, useState } from "react";
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
import { Label } from "@/components/ui/label";
import { RadioGroup, RadioGroupItem } from "@/components/ui/radio-group";
import {
	Table,
	TableBody,
	TableCell,
	TableHead,
	TableHeader,
	TableRow,
} from "@/components/ui/table";
import {
	ConflictResolution,
	type ImportError,
} from "@/gen/elara/transfer/v1/transfer_pb";
import { importNamespace } from "@/gen/elara/transfer/v1/transfer_service-TransferService_connectquery";

interface PreviewRow {
	path: string;
	namespace: string;
	action: string;
}

interface ImportDialogProps {
	namespace?: string;
}

export function ImportDialog({ namespace: _namespace }: ImportDialogProps) {
	const [open, setOpen] = useState(false);
	const [fileBytes, setFileBytes] = useState<Uint8Array | null>(null);
	const [fileName, setFileName] = useState("");
	const [onConflict, setOnConflict] = useState<ConflictResolution>(
		ConflictResolution.SKIP,
	);
	const [preview, setPreview] = useState<PreviewRow[] | null>(null);
	const [previewErrors, setPreviewErrors] = useState<ImportError[]>([]);
	const inputRef = useRef<HTMLInputElement>(null);

	const mutation = useMutation(importNamespace);
	const previewMutation = useMutation(importNamespace);

	const handleFileChange = (e: React.ChangeEvent<HTMLInputElement>) => {
		const file = e.target.files?.[0];
		if (!file) return;

		setFileName(file.name);
		setPreview(null);
		setPreviewErrors([]);

		const reader = new FileReader();
		reader.onload = (ev) => {
			const buf = ev.target?.result as ArrayBuffer;
			setFileBytes(new Uint8Array(buf));
		};
		reader.readAsArrayBuffer(file);
	};

	const handlePreview = () => {
		if (!fileBytes) return;

		previewMutation.mutate(
			{ data: fileBytes, onConflict, dryRun: true },
			{
				onSuccess: (res) => {
					const rows: PreviewRow[] = [];

					for (const err of res.errors) {
						rows.push({
							path: err.path,
							namespace: err.namespace,
							action: `fail: ${err.message}`,
						});
					}

					if (res.created > 0) {
						rows.push({
							path: `${res.created} config(s)`,
							namespace: "",
							action:
								onConflict === ConflictResolution.OVERWRITE
									? "overwrite / create"
									: "create",
						});
					}

					if (res.skipped > 0) {
						rows.push({
							path: `${res.skipped} config(s)`,
							namespace: "",
							action: "skip (exists)",
						});
					}

					setPreview(rows);
					setPreviewErrors(res.errors);
				},
				onError: (err) => toast.error(err.message),
			},
		);
	};

	const resetState = () => {
		setFileBytes(null);
		setFileName("");
		setPreview(null);
		setPreviewErrors([]);
		if (inputRef.current) inputRef.current.value = "";
	};

	const handleImport = () => {
		if (!fileBytes) return;

		mutation.mutate(
			{ data: fileBytes, onConflict, dryRun: false },
			{
				onSuccess: (res) => {
					toast.success(
						`Import complete — Created: ${res.created}, Skipped: ${res.skipped}, Failed: ${res.failed}`,
					);
					setOpen(false);
					resetState();
				},
				onError: (err) => toast.error(err.message),
			},
		);
	};

	const handleOpenChange = (isOpen: boolean) => {
		if (!isOpen) resetState();
		setOpen(isOpen);
	};

	return (
		<Dialog open={open} onOpenChange={handleOpenChange}>
			<DialogTrigger render={<Button variant="outline" size="sm" />}>
				<Upload className="mr-1 h-4 w-4" />
				Import
			</DialogTrigger>
			<DialogContent className="max-w-2xl">
				<DialogHeader>
					<DialogTitle>Import Configs</DialogTitle>
				</DialogHeader>

				<div className="grid gap-4 py-4">
					<Field>
						<FieldLabel>Upload file (.json, .yaml, .yml, .zip)</FieldLabel>
						<input
							ref={inputRef}
							type="file"
							accept=".json,.yaml,.yml,.zip"
							onChange={handleFileChange}
							className="text-sm file:mr-3 file:rounded-md file:border file:border-input file:bg-background file:px-3 file:py-1 file:text-sm file:font-medium"
						/>
						{fileName && (
							<p className="text-muted-foreground text-sm">
								Selected: {fileName}
							</p>
						)}
					</Field>

					<Field>
						<FieldLabel>Conflict resolution</FieldLabel>
						<RadioGroup
							value={String(onConflict)}
							onValueChange={(v) =>
								setOnConflict(Number(v) as ConflictResolution)
							}
							className="flex gap-4"
						>
							<div className="flex items-center gap-2">
								<RadioGroupItem
									value={String(ConflictResolution.SKIP)}
									id="cr-skip"
								/>
								<Label htmlFor="cr-skip">Skip existing</Label>
							</div>
							<div className="flex items-center gap-2">
								<RadioGroupItem
									value={String(ConflictResolution.OVERWRITE)}
									id="cr-overwrite"
								/>
								<Label htmlFor="cr-overwrite">Overwrite</Label>
							</div>
							<div className="flex items-center gap-2">
								<RadioGroupItem
									value={String(ConflictResolution.FAIL)}
									id="cr-fail"
								/>
								<Label htmlFor="cr-fail">Fail on conflict</Label>
							</div>
						</RadioGroup>
					</Field>

					{fileBytes && (
						<Button
							variant="outline"
							size="sm"
							onClick={handlePreview}
							disabled={previewMutation.isPending}
							className="w-fit"
						>
							{previewMutation.isPending ? "Loading preview..." : "Preview"}
						</Button>
					)}

					{preview !== null && (
						<div className="max-h-64 overflow-auto rounded-md border">
							<Table>
								<TableHeader>
									<TableRow>
										<TableHead>Path / Summary</TableHead>
										<TableHead>Namespace</TableHead>
										<TableHead>Action</TableHead>
									</TableRow>
								</TableHeader>
								<TableBody>
									{preview.length === 0 ? (
										<TableRow>
											<TableCell
												colSpan={3}
												className="text-muted-foreground text-center"
											>
												Nothing to import
											</TableCell>
										</TableRow>
									) : (
										preview.map((row, i) => (
											// biome-ignore lint/suspicious/noArrayIndexKey: preview rows are ephemeral
											<TableRow key={i}>
												<TableCell className="font-mono text-xs">
													{row.path}
												</TableCell>
												<TableCell className="text-xs">
													{row.namespace}
												</TableCell>
												<TableCell className="text-xs">{row.action}</TableCell>
											</TableRow>
										))
									)}
								</TableBody>
							</Table>
						</div>
					)}

					{previewErrors.length > 0 && (
						<p className="text-destructive text-sm">
							{previewErrors.length} conflict(s) detected
						</p>
					)}
				</div>

				<DialogFooter>
					<Button
						onClick={handleImport}
						disabled={!fileBytes || mutation.isPending}
					>
						{mutation.isPending ? "Importing..." : "Import"}
					</Button>
				</DialogFooter>
			</DialogContent>
		</Dialog>
	);
}

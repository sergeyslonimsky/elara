import { useMutation } from "@connectrpc/connect-query";
import { Download } from "lucide-react";
import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
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
import { BundleEncoding, ZipLayout } from "@/gen/elara/transfer/v1/transfer_pb";
import {
	exportAll,
	exportNamespace,
} from "@/gen/elara/transfer/v1/transfer_service-TransferService_connectquery";
import { triggerDownload } from "@/lib/download";
import { toastError } from "@/lib/toast";

interface ExportDialogProps {
	namespace?: string;
}

export function ExportDialog({ namespace }: ExportDialogProps) {
	const [open, setOpen] = useState(false);
	const [encoding, setEncoding] = useState<BundleEncoding>(BundleEncoding.JSON);
	const [zip, setZip] = useState(false);
	const [zipLayout, setZipLayout] = useState<ZipLayout>(ZipLayout.BUNDLE);

	const ext = zip
		? ".zip"
		: encoding === BundleEncoding.YAML
			? ".yaml"
			: ".json";
	const previewFilename = namespace
		? `${namespace}-export${ext}`
		: `elara-export-all${ext}`;

	// Both export RPCs return the same bundle-shaped response (data, filename,
	// contentType), so they share onSuccess/onError handlers.
	const handleDownload = (res: {
		data: Uint8Array;
		filename: string;
		contentType: string;
	}) => {
		triggerDownload(
			res.data,
			res.filename || previewFilename,
			res.contentType || "application/octet-stream",
		);
		setOpen(false);
	};

	const exportNsMutation = useMutation(exportNamespace, {
		onSuccess: handleDownload,
		onError: toastError,
	});

	const exportAllMutation = useMutation(exportAll, {
		onSuccess: handleDownload,
		onError: toastError,
	});

	const isPending = exportNsMutation.isPending || exportAllMutation.isPending;

	const handleExport = () => {
		if (namespace) {
			exportNsMutation.mutate({ namespace, zip, encoding });
		} else {
			exportAllMutation.mutate({ zip, encoding, zipLayout });
		}
	};

	return (
		<Dialog open={open} onOpenChange={setOpen}>
			<DialogTrigger render={<Button variant="outline" size="sm" />}>
				<Download className="mr-1 h-4 w-4" />
				{namespace ? "Export" : "Export All"}
			</DialogTrigger>
			<DialogContent>
				<DialogHeader>
					<DialogTitle>
						{namespace
							? `Export Namespace: ${namespace}`
							: "Export All Namespaces"}
					</DialogTitle>
				</DialogHeader>

				<div className="grid gap-4 py-4">
					<Field>
						<FieldLabel>Bundle format</FieldLabel>
						<RadioGroup
							value={String(encoding)}
							onValueChange={(v) => setEncoding(Number(v) as BundleEncoding)}
							className="flex gap-4"
						>
							<div className="flex items-center gap-2">
								<RadioGroupItem
									value={String(BundleEncoding.JSON)}
									id="enc-json"
								/>
								<Label htmlFor="enc-json">JSON</Label>
							</div>
							<div className="flex items-center gap-2">
								<RadioGroupItem
									value={String(BundleEncoding.YAML)}
									id="enc-yaml"
								/>
								<Label htmlFor="enc-yaml">YAML</Label>
							</div>
						</RadioGroup>
					</Field>

					<Field>
						<div className="flex items-center gap-2">
							<Checkbox
								id="zip-check"
								checked={zip}
								onCheckedChange={(v) => setZip(Boolean(v))}
							/>
							<Label htmlFor="zip-check">Compress as ZIP</Label>
						</div>
					</Field>

					{zip && !namespace && (
						<Field>
							<FieldLabel>ZIP layout</FieldLabel>
							<RadioGroup
								value={String(zipLayout)}
								onValueChange={(v) => setZipLayout(Number(v) as ZipLayout)}
								className="flex flex-col gap-2"
							>
								<div className="flex items-center gap-2">
									<RadioGroupItem
										value={String(ZipLayout.BUNDLE)}
										id="zip-bundle"
									/>
									<Label htmlFor="zip-bundle">
										Single bundle file (default)
									</Label>
								</div>
								<div className="flex items-center gap-2">
									<RadioGroupItem
										value={String(ZipLayout.PER_NAMESPACE)}
										id="zip-per-ns"
									/>
									<Label htmlFor="zip-per-ns">One file per namespace</Label>
								</div>
							</RadioGroup>
						</Field>
					)}

					<p className="text-muted-foreground text-sm">
						Filename preview:{" "}
						<span className="font-mono">{previewFilename}</span>
					</p>
				</div>

				<DialogFooter>
					<Button onClick={handleExport} disabled={isPending}>
						{isPending ? "Exporting..." : "Download"}
					</Button>
				</DialogFooter>
			</DialogContent>
		</Dialog>
	);
}

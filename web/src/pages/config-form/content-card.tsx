import { useMutation } from "@connectrpc/connect-query";
import { CheckCircle, Sparkles } from "lucide-react";
import { useState } from "react";
import { toast } from "sonner";
import { ConfigEditor } from "@/components/config-editor";
import { Button } from "@/components/ui/button";
import {
	Card,
	CardContent,
	CardDescription,
	CardHeader,
} from "@/components/ui/card";
import { Field, FieldLabel } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import {
	Select,
	SelectContent,
	SelectItem,
	SelectTrigger,
	SelectValue,
} from "@/components/ui/select";
import type { Format } from "@/gen/elara/config/v1/config_pb";
import { validateConfig } from "@/gen/elara/config/v1/config_service-ConfigService_connectquery";
import { formatToLanguage } from "@/lib/format";
import { toastError } from "@/lib/toast";

interface ContentCardProps {
	isEdit: boolean;
	namespace: string;
	configPath: string;
	parentPath: string;
	fullPath: string;
	filename: string;
	onFilenameChange: (v: string) => void;
	format: string;
	onFormatChange: (v: string) => void;
	content: string;
	onContentChange: (v: string) => void;
	protoFormat: Format;
}

export function ContentCard({
	isEdit,
	namespace,
	configPath,
	parentPath,
	fullPath,
	filename,
	onFilenameChange,
	format,
	onFormatChange,
	content,
	onContentChange,
	protoFormat,
}: Readonly<ContentCardProps>) {
	const [validationErrors, setValidationErrors] = useState<string[]>([]);

	const formatMutation = useMutation(validateConfig, {
		onSuccess: (res) => {
			if (res.result?.valid && res.result.normalizedContent) {
				onContentChange(res.result.normalizedContent);
				toast.success("Content formatted");
				setValidationErrors([]);
			} else {
				setValidationErrors(res.result?.errors ?? []);
				toast.error("Cannot format — content has errors");
			}
		},
		onError: toastError,
	});

	const validateMutation = useMutation(validateConfig, {
		onSuccess: (res) => {
			if (res.result?.valid) {
				toast.success("Content is valid");
				setValidationErrors([]);
			} else {
				setValidationErrors(res.result?.errors ?? []);
				toast.error("Validation failed");
			}
		},
		onError: toastError,
	});

	const canValidate =
		format !== "auto" && format !== "other" && content.length > 0;
	const editorLanguage = formatToLanguage(
		format === "auto" ? "plaintext" : format,
	);

	return (
		<Card className="rounded-xl">
			<CardHeader>
				<CardDescription>
					{isEdit ? (
						<>
							Editing{" "}
							<code className="rounded bg-muted px-1 text-xs">
								{namespace}:{configPath}
							</code>
						</>
					) : (
						<>
							Creating in{" "}
							<code className="rounded bg-muted px-1 text-xs">
								{namespace}:{parentPath}
							</code>
						</>
					)}
				</CardDescription>
			</CardHeader>
			<CardContent className="space-y-4">
				<div className="grid gap-4 sm:grid-cols-2">
					<Field>
						<FieldLabel>Filename</FieldLabel>
						<Input
							value={filename}
							onChange={(e) => onFilenameChange(e.target.value)}
							placeholder="config.json"
							disabled={isEdit}
							required
						/>
						{!isEdit && (
							<p className="text-muted-foreground text-xs">Path: {fullPath}</p>
						)}
					</Field>
					<Field>
						<FieldLabel>Format</FieldLabel>
						<Select
							value={format}
							onValueChange={(v) => onFormatChange(v ?? "auto")}
						>
							<SelectTrigger>
								<SelectValue />
							</SelectTrigger>
							<SelectContent>
								{!isEdit && (
									<SelectItem value="auto">
										Auto-detect from extension
									</SelectItem>
								)}
								<SelectItem value="json">JSON</SelectItem>
								<SelectItem value="yaml">YAML</SelectItem>
								<SelectItem value="other">Other (raw)</SelectItem>
							</SelectContent>
						</Select>
					</Field>
				</div>

				<Field>
					<div className="flex items-center justify-between">
						<FieldLabel>Content</FieldLabel>
						<div className="flex gap-2">
							<Button
								type="button"
								variant="outline"
								size="xs"
								onClick={() =>
									formatMutation.mutate({
										content,
										format: protoFormat,
									})
								}
								disabled={!canValidate || formatMutation.isPending}
							>
								<Sparkles className="mr-1 h-3 w-3" />
								{formatMutation.isPending ? "Formatting..." : "Format"}
							</Button>
							<Button
								type="button"
								variant="outline"
								size="xs"
								onClick={() =>
									validateMutation.mutate({
										content,
										format: protoFormat,
									})
								}
								disabled={!canValidate || validateMutation.isPending}
							>
								<CheckCircle className="mr-1 h-3 w-3" />
								Validate
							</Button>
						</div>
					</div>
					<ConfigEditor
						value={content}
						onChange={(v) => {
							onContentChange(v);
							setValidationErrors([]);
						}}
						language={editorLanguage}
					/>
					{validationErrors.length > 0 && (
						<div className="rounded-md bg-destructive/10 p-3 text-destructive text-sm">
							{validationErrors.map((err) => (
								<p key={err}>{err}</p>
							))}
						</div>
					)}
				</Field>
			</CardContent>
		</Card>
	);
}

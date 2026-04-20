import { useMutation, useQuery } from "@connectrpc/connect-query";
import { useQueryClient } from "@tanstack/react-query";
import { ArrowLeft, CheckCircle, Sparkles } from "lucide-react";
import { useEffect, useState } from "react";
import { Link, useLocation, useNavigate, useParams } from "react-router";
import { toast } from "sonner";
import { ConfigEditor } from "@/components/config-editor";
import { PageHeader } from "@/components/page-header.tsx";
import { PathBreadcrumb } from "@/components/path-breadcrumb";
import { Button } from "@/components/ui/button";
import {
	Card,
	CardContent,
	CardDescription,
	CardHeader,
	CardTitle,
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
import { Separator } from "@/components/ui/separator";
import { Format } from "@/gen/elara/config/v1/config_pb";
import {
	createConfig,
	getConfig,
	updateConfig,
	validateConfig,
} from "@/gen/elara/config/v1/config_service-ConfigService_connectquery";
import { formatToLanguage } from "@/lib/format";
import { invalidateConfig, invalidateConfigs } from "@/lib/queries";

function formatToString(f: Format): string {
	switch (f) {
		case Format.JSON:
			return "json";
		case Format.YAML:
			return "yaml";
		case Format.OTHER:
			return "other";
		default:
			return "auto";
	}
}

function stringToProtoFormat(s: string): Format {
	switch (s) {
		case "json":
			return Format.JSON;
		case "yaml":
			return Format.YAML;
		case "other":
			return Format.OTHER;
		default:
			return Format.UNSPECIFIED;
	}
}

export function ConfigFormPage() {
	const { namespace = "default", "*": splat = "" } = useParams();
	const currentPath = splat ? `/${splat}` : "/";
	const navigate = useNavigate();
	const queryClient = useQueryClient();

	const location = useLocation();

	// Determine if editing existing or creating new.
	const isEdit = location.pathname.startsWith("/config/edit/");

	// For edit: path IS the config path. For create: path is the parent folder.
	const configPath = isEdit ? currentPath : "";
	const parentPath = isEdit
		? currentPath.split("/").slice(0, -1).join("/") || "/"
		: currentPath;

	const [filename, setFilename] = useState("");
	const [content, setContent] = useState("");
	const [format, setFormat] = useState("auto");
	const [metaKey, setMetaKey] = useState("");
	const [metaValue, setMetaValue] = useState("");
	const [metadata, setMetadata] = useState<Record<string, string>>({});
	const [version, setVersion] = useState<bigint>(0n);
	const [validationErrors, setValidationErrors] = useState<string[]>([]);
	const [loaded, setLoaded] = useState(!isEdit);

	// Reset loaded flag when switching between different configs in edit mode.
	// biome-ignore lint/correctness/useExhaustiveDependencies: intentional reset on route change
	useEffect(() => {
		setLoaded(!isEdit);
	}, [isEdit, configPath, namespace]);

	// Load existing config for edit mode.
	const { data: existing } = useQuery(
		getConfig,
		isEdit ? { path: configPath, namespace } : undefined,
	);

	useEffect(() => {
		if (isEdit && existing?.config && !loaded) {
			setFilename(existing.config.path.split("/").pop() ?? "");
			setContent(existing.config.content);
			setFormat(formatToString(existing.config.format));
			setVersion(existing.config.version);
			setMetadata(
				existing.config.metadata
					? Object.fromEntries(Object.entries(existing.config.metadata))
					: {},
			);
			setLoaded(true);
		}
	}, [isEdit, existing, loaded]);

	const fullPath = isEdit
		? configPath
		: parentPath === "/"
			? `/${filename}`
			: `${parentPath}/${filename}`;

	const protoFormat = stringToProtoFormat(format);
	const editorLanguage = formatToLanguage(
		format === "auto" ? "plaintext" : format,
	);

	const createMutation = useMutation(createConfig, {
		onSuccess: () => {
			toast.success(`Config "${fullPath}" created`);
			invalidateAndNavigate();
		},
		onError: (err) => toast.error(err.message),
	});

	const updateMutation = useMutation(updateConfig, {
		onSuccess: () => {
			toast.success("Config updated");
			invalidateAndNavigate();
		},
		onError: (err) => toast.error(err.message),
	});

	const formatMutation = useMutation(validateConfig, {
		onSuccess: (res) => {
			if (res.result?.valid && res.result.normalizedContent) {
				setContent(res.result.normalizedContent);
				toast.success("Content formatted");
				setValidationErrors([]);
			} else {
				setValidationErrors(res.result?.errors ?? []);
				toast.error("Cannot format — content has errors");
			}
		},
		onError: (err) => toast.error(err.message),
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
		onError: (err) => toast.error(err.message),
	});

	function invalidateAndNavigate() {
		void invalidateConfigs(queryClient);
		void invalidateConfig(queryClient);
		navigate(
			isEdit
				? `/config/${namespace}${configPath}`
				: `/browse/${namespace}${parentPath}`,
		);
	}

	const handleSubmit = (e: React.FormEvent) => {
		e.preventDefault();
		if (isEdit) {
			updateMutation.mutate({
				path: fullPath,
				namespace,
				content,
				format: protoFormat,
				version,
				metadata,
			});
		} else {
			createMutation.mutate({
				path: fullPath,
				content,
				namespace,
				format: protoFormat,
				metadata,
			});
		}
	};

	const canValidate =
		format !== "auto" && format !== "other" && content.length > 0;
	const isPending = createMutation.isPending || updateMutation.isPending;
	const backTo = isEdit
		? `/config/${namespace}${configPath}`
		: `/browse/${namespace}${parentPath}`;

	const handleAddMeta = () => {
		if (metaKey && metaValue) {
			setMetadata((prev) => ({ ...prev, [metaKey]: metaValue }));
			setMetaKey("");
			setMetaValue("");
		}
	};

	return (
		<>
			<PageHeader title="Edit Config" />
			<div className="flex flex-1 flex-col gap-4 p-4 pt-0">
				<div className="mt-4 flex items-center gap-4">
					<Button variant="ghost" size="sm" render={<Link to={backTo} />}>
						<ArrowLeft className="mr-1 h-4 w-4" />
						Back
					</Button>
					<PathBreadcrumb
						namespace={namespace}
						path={isEdit ? configPath : parentPath}
					/>
				</div>

				<form onSubmit={handleSubmit} className="grid gap-4 lg:grid-cols-3">
					<div className="space-y-4 lg:col-span-2">
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
											onChange={(e) => setFilename(e.target.value)}
											placeholder="config.json"
											disabled={isEdit}
											required
										/>
										{!isEdit && (
											<p className="text-muted-foreground text-xs">
												Path: {fullPath}
											</p>
										)}
									</Field>
									<Field>
										<FieldLabel>Format</FieldLabel>
										<Select
											value={format}
											onValueChange={(v) => setFormat(v ?? "auto")}
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
											setContent(v);
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
					</div>

					<div className="space-y-4">
						<Card className="rounded-xl">
							<CardHeader>
								<CardTitle className="text-sm">Metadata</CardTitle>
								<CardDescription>Optional key-value pairs</CardDescription>
							</CardHeader>
							<CardContent className="space-y-3">
								<div className="flex gap-2">
									<Input
										value={metaKey}
										onChange={(e) => setMetaKey(e.target.value)}
										placeholder="Key"
										className="flex-1"
									/>
									<Input
										value={metaValue}
										onChange={(e) => setMetaValue(e.target.value)}
										onKeyDown={(e) => {
											if (e.key === "Enter") {
												e.preventDefault();
												handleAddMeta();
											}
										}}
										placeholder="Value"
										className="flex-1"
									/>
									<Button
										type="button"
										variant="outline"
										size="sm"
										onClick={handleAddMeta}
										disabled={!metaKey || !metaValue}
									>
										Add
									</Button>
								</div>
								{Object.entries(metadata).map(([key, value]) => (
									<div
										key={key}
										className="flex items-center justify-between rounded-md bg-muted px-3 py-1.5 text-sm"
									>
										<span>
											<strong>{key}</strong>: {value}
										</span>
										<Button
											type="button"
											variant="ghost"
											size="xs"
											onClick={() =>
												setMetadata((prev) => {
													const next = { ...prev };
													delete next[key];
													return next;
												})
											}
										>
											×
										</Button>
									</div>
								))}
								{Object.keys(metadata).length === 0 && (
									<p className="text-muted-foreground text-xs">
										No metadata added
									</p>
								)}
							</CardContent>
						</Card>

						<Separator />

						<Button
							type="submit"
							className="w-full"
							disabled={isPending || !filename || !content}
						>
							{isPending
								? isEdit
									? "Saving..."
									: "Creating..."
								: isEdit
									? "Save Changes"
									: "Create Config"}
						</Button>
					</div>
				</form>
			</div>
		</>
	);
}

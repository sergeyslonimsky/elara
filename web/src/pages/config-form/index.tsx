import { useMutation, useQuery } from "@connectrpc/connect-query";
import { useQueryClient } from "@tanstack/react-query";
import { ArrowLeft } from "lucide-react";
import { useEffect, useState } from "react";
import { Link, useLocation, useNavigate, useParams } from "react-router";
import { toast } from "sonner";
import { PageShell } from "@/components/page-shell";
import { PathBreadcrumb } from "@/components/path-breadcrumb";
import { Button } from "@/components/ui/button";
import { Separator } from "@/components/ui/separator";
import {
	createConfig,
	getConfig,
	updateConfig,
} from "@/gen/elara/config/v1/config_service-ConfigService_connectquery";
import { parentPathOf } from "@/hooks/use-back-link";
import { formatToString, stringToProtoFormat } from "@/lib/format";
import { invalidateAllConfigData } from "@/lib/queries";
import { toastError } from "@/lib/toast";
import { ContentCard } from "./content-card";
import { MetadataEditor } from "./metadata-editor";

export function ConfigFormPage() {
	const { namespace: namespaceParam, "*": splat = "" } = useParams();
	const namespace = namespaceParam ?? "";
	const currentPath = splat ? `/${splat}` : "/";
	const navigate = useNavigate();
	const queryClient = useQueryClient();

	const location = useLocation();

	const isEdit = location.pathname.startsWith("/config/edit/");

	const configPath = isEdit ? currentPath : "";
	const parentPath = isEdit ? parentPathOf(currentPath) : currentPath;

	const [filename, setFilename] = useState("");
	const [content, setContent] = useState("");
	const [format, setFormat] = useState("auto");
	const [metadata, setMetadata] = useState<Record<string, string>>({});
	const [version, setVersion] = useState<bigint>(0n);
	const [loaded, setLoaded] = useState(!isEdit);

	// biome-ignore lint/correctness/useExhaustiveDependencies: intentional reset on route change
	useEffect(() => {
		setLoaded(!isEdit);
	}, [isEdit, configPath, namespace]);

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

	const createMutation = useMutation(createConfig, {
		onSuccess: () => {
			toast.success(`Config "${fullPath}" created`);
			invalidateAndNavigate();
		},
		onError: toastError,
	});

	const updateMutation = useMutation(updateConfig, {
		onSuccess: () => {
			toast.success("Config updated");
			invalidateAndNavigate();
		},
		onError: toastError,
	});

	function invalidateAndNavigate() {
		void invalidateAllConfigData(queryClient);
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

	const isPending = createMutation.isPending || updateMutation.isPending;
	const backTo = isEdit
		? `/config/${namespace}${configPath}`
		: `/browse/${namespace}${parentPath}`;

	return (
		<PageShell title={isEdit ? "Edit Config" : "New Config"}>
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
					<ContentCard
						isEdit={isEdit}
						namespace={namespace}
						configPath={configPath}
						parentPath={parentPath}
						fullPath={fullPath}
						filename={filename}
						onFilenameChange={setFilename}
						format={format}
						onFormatChange={setFormat}
						content={content}
						onContentChange={setContent}
						protoFormat={protoFormat}
					/>
				</div>

				<div className="space-y-4">
					<MetadataEditor metadata={metadata} onChange={setMetadata} />

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
		</PageShell>
	);
}

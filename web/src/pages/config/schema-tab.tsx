import { useQuery } from "@connectrpc/connect-query";
import Ajv from "ajv";
import addFormats from "ajv-formats";
import { AlertCircle, ExternalLink, Info } from "lucide-react";
import { useMemo } from "react";
import { Link } from "react-router";
import { ConfigEditor } from "@/components/config-editor";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { getEffectiveSchema } from "@/gen/elara/config/v1/schema_service-SchemaService_connectquery";
import { SchemaValidationResult } from "./schema-validation-result";

const ajv = new Ajv({ allErrors: true });
addFormats(ajv);

interface SchemaViolation {
	path: string;
	message: string;
	keyword: string;
}

interface SchemaTabProps {
	namespace: string;
	path: string;
	configContent: string;
	language: string;
}

export function SchemaTab({
	namespace,
	path,
	configContent,
	language,
}: Readonly<SchemaTabProps>) {
	const { data: schemaData } = useQuery(getEffectiveSchema, {
		namespace,
		path,
	});

	const schema = schemaData?.schema;
	const schemaJson = schema?.jsonSchema ?? "";
	const matchedPattern = schema?.pathPattern ?? "";

	const violations: SchemaViolation[] = useMemo(() => {
		if (language !== "json" || !schemaJson.trim() || !configContent.trim())
			return [];
		try {
			const schemaObj = JSON.parse(schemaJson);
			const dataObj = JSON.parse(configContent);
			const validate = ajv.compile(schemaObj);
			const valid = validate(dataObj);
			if (valid) return [];
			return (validate.errors ?? []).map((e) => ({
				path: e.instancePath || "/",
				message: e.message ?? "validation error",
				keyword: e.keyword,
			}));
		} catch {
			return [];
		}
	}, [schemaJson, configContent, language]);

	if (!schema) {
		return (
			<Card className="rounded-xl">
				<CardContent className="pt-6">
					<p className="text-sm text-muted-foreground">
						No schema applied to this config.{" "}
						<Link
							to={`/namespaces/${namespace}/schemas`}
							className="inline-flex items-center gap-1 underline underline-offset-2"
						>
							Manage schemas
							<ExternalLink className="h-3 w-3" />
						</Link>
					</p>
				</CardContent>
			</Card>
		);
	}

	return (
		<div className="space-y-4">
			<Card className="rounded-xl">
				<CardHeader className="pb-2">
					<div className="flex items-center justify-between">
						<div className="flex items-center gap-2">
							<CardTitle className="text-sm">Applied Schema</CardTitle>
							<span className="font-mono text-xs text-muted-foreground">
								{matchedPattern}
							</span>
							{matchedPattern !== path && (
								<span className="text-xs text-muted-foreground">
									(inherited)
								</span>
							)}
						</div>
						<Link
							to={`/namespaces/${namespace}/schemas`}
							className="inline-flex items-center gap-1 text-xs text-muted-foreground underline underline-offset-2"
						>
							Manage schemas
							<ExternalLink className="h-3 w-3" />
						</Link>
					</div>
				</CardHeader>
				<CardContent>
					<ConfigEditor
						value={schemaJson}
						onChange={() => {}}
						language="json"
						height="300px"
						readOnly
					/>
				</CardContent>
			</Card>

			{language === "yaml" && (
				<div className="flex items-start gap-2 rounded-lg border border-blue-200 bg-blue-50 px-4 py-3 text-sm text-blue-800 dark:border-blue-800 dark:bg-blue-950 dark:text-blue-200">
					<Info className="mt-0.5 h-4 w-4 shrink-0" />
					<span>
						Live validation is not available for YAML. Schema validation runs
						server-side on save.
					</span>
				</div>
			)}

			{language !== "json" && language !== "yaml" && (
				<div className="flex items-start gap-2 rounded-lg border border-yellow-200 bg-yellow-50 px-4 py-3 text-sm text-yellow-800 dark:border-yellow-800 dark:bg-yellow-950 dark:text-yellow-200">
					<AlertCircle className="mt-0.5 h-4 w-4 shrink-0" />
					<span>Schema validation is skipped for non-JSON/YAML formats.</span>
				</div>
			)}

			{language === "json" && schemaJson.trim() && (
				<Card className="rounded-xl">
					<CardHeader className="pb-2">
						<CardTitle className="text-sm">Live Validation</CardTitle>
					</CardHeader>
					<CardContent>
						<SchemaValidationResult violations={violations} />
					</CardContent>
				</Card>
			)}
		</div>
	);
}

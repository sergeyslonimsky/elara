import { useMutation, useQuery } from "@connectrpc/connect-query";
import { useQueryClient } from "@tanstack/react-query";
import { ArrowLeft, Trash2 } from "lucide-react";
import { Link, useParams } from "react-router";
import { toast } from "sonner";
import { ErrorCard } from "@/components/error-card";
import { PageShell } from "@/components/page-shell";
import {
	AlertDialog,
	AlertDialogAction,
	AlertDialogCancel,
	AlertDialogContent,
	AlertDialogDescription,
	AlertDialogFooter,
	AlertDialogHeader,
	AlertDialogTitle,
	AlertDialogTrigger,
} from "@/components/ui/alert-dialog";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import {
	detachSchema,
	listSchemas,
} from "@/gen/elara/config/v1/schema_service-SchemaService_connectquery";
import { getNamespace } from "@/gen/elara/namespace/v1/namespace_service-NamespaceService_connectquery";
import { invalidate } from "@/lib/queries";
import { toastError } from "@/lib/toast";
import { AttachSchemaDialog } from "./attach-schema-dialog";

export function SchemasPage() {
	const { name: namespace = "" } = useParams();

	const { data: nsData } = useQuery(getNamespace, { name: namespace });
	const namespaceLocked = nsData?.namespace?.locked ?? false;

	const { data, isLoading, error } = useQuery(listSchemas, { namespace });
	const queryClient = useQueryClient();

	const detachMutation = useMutation(detachSchema, {
		onSuccess: () => {
			toast.success("Schema detached");
			invalidate(queryClient, "schemas");
		},
		onError: toastError,
	});

	const schemas = data?.schemas ?? [];

	return (
		<PageShell title={`Schemas — ${namespace}`}>
			<div>
				<Button variant="ghost" size="sm" render={<Link to="/namespaces" />}>
					<ArrowLeft className="mr-1 h-4 w-4" />
					Back to Namespaces
				</Button>
			</div>

			<div className="flex justify-end">
				<AttachSchemaDialog namespace={namespace} disabled={namespaceLocked} />
			</div>

			{isLoading && (
				<div className="space-y-2">
					<Skeleton className="h-16 rounded-xl" />
					<Skeleton className="h-16 rounded-xl" />
				</div>
			)}

			{error && <ErrorCard message={error.message} />}

			{!isLoading && schemas.length === 0 && (
				<p className="text-sm text-muted-foreground">
					No schemas attached in this namespace.
				</p>
			)}

			<div className="space-y-2">
				{schemas.map((s) => (
					<Card key={s.id} className="rounded-xl">
						<CardContent className="flex items-start justify-between pt-4">
							<div className="space-y-1">
								<p className="font-mono text-sm font-medium">{s.pathPattern}</p>
								<p className="line-clamp-1 text-xs text-muted-foreground">
									{s.jsonSchema}
								</p>
								{s.createdAt && (
									<p className="text-xs text-muted-foreground">
										Attached{" "}
										{new Date(
											Number(s.createdAt.seconds) * 1000,
										).toLocaleDateString()}
									</p>
								)}
							</div>
							<div className="flex gap-1">
								<AttachSchemaDialog
									namespace={namespace}
									initialPathPattern={s.pathPattern}
									initialJsonSchema={s.jsonSchema}
									disabled={namespaceLocked}
								/>
								<AlertDialog>
									<AlertDialogTrigger
										render={
											<Button
												variant="ghost"
												size="icon-xs"
												disabled={namespaceLocked}
											/>
										}
									>
										<Trash2 className="h-3.5 w-3.5 text-destructive" />
									</AlertDialogTrigger>
									<AlertDialogContent>
										<AlertDialogHeader>
											<AlertDialogTitle>Detach schema?</AlertDialogTitle>
											<AlertDialogDescription>
												This will remove the schema from pattern "
												{s.pathPattern}".
											</AlertDialogDescription>
										</AlertDialogHeader>
										<AlertDialogFooter>
											<AlertDialogCancel>Cancel</AlertDialogCancel>
											<AlertDialogAction
												className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
												disabled={detachMutation.isPending}
												onClick={() =>
													detachMutation.mutate({
														namespace,
														pathPattern: s.pathPattern,
													})
												}
											>
												{detachMutation.isPending ? "Detaching..." : "Detach"}
											</AlertDialogAction>
										</AlertDialogFooter>
									</AlertDialogContent>
								</AlertDialog>
							</div>
						</CardContent>
					</Card>
				))}
			</div>
		</PageShell>
	);
}

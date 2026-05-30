import { subject } from "@casl/ability";
import { Database, Lock, ShieldCheck } from "lucide-react";
import { Link } from "react-router";
import { useAbility } from "@/auth/ability-context";
import { ExportDialog } from "@/components/export-dialog";
import { ImportDialog } from "@/components/import-dialog";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
	Card,
	CardContent,
	CardDescription,
	CardHeader,
	CardTitle,
} from "@/components/ui/card";
import type { Namespace } from "@/gen/elara/namespace/v1/namespace_pb";
import { DeleteButton } from "./delete-button";
import { EditDialog } from "./edit-dialog";
import { LockButton } from "./lock-button";

export function NamespaceCard({ ns }: Readonly<{ ns: Namespace }>) {
	const ability = useAbility();
	const canWrite = ability.can(
		"write",
		subject("Namespace", { domain: ns.name }),
	);
	// Deleting the namespace itself is a distinct capability from editing its
	// content (configs). Only a delete grant (or *) exposes the delete action.
	const canDelete = ability.can(
		"delete",
		subject("Namespace", { domain: ns.name }),
	);

	return (
		<Card className="rounded-xl">
			<CardHeader className="pb-2">
				<div className="flex items-center justify-between">
					<Link
						to={`/browse/${ns.name}`}
						className="flex items-center gap-2 hover:underline"
					>
						<Database className="h-4 w-4 text-muted-foreground" />
						<CardTitle className="text-base">{ns.name}</CardTitle>
						{ns.locked && <Lock className="h-3 w-3 text-amber-500" />}
					</Link>
					{(canWrite || canDelete) && (
						<div className="flex gap-1">
							{canWrite && (
								<EditDialog
									name={ns.name}
									currentDescription={ns.description}
									locked={ns.locked}
								/>
							)}
							{canDelete && <DeleteButton name={ns.name} locked={ns.locked} />}
						</div>
					)}
				</div>
				<CardDescription className="min-h-[1.25rem]">
					{ns.description}
				</CardDescription>
			</CardHeader>
			<CardContent className="space-y-2">
				<Badge variant="secondary">
					{ns.configCount} config{ns.configCount === 1 ? "" : "s"}
				</Badge>
				<div className="flex flex-wrap gap-2">
					<ExportDialog namespace={ns.name} />
					<ImportDialog namespace={ns.name} />
					{canWrite && <LockButton name={ns.name} locked={ns.locked} />}
					<Button
						size="sm"
						variant="outline"
						render={<Link to={`/namespaces/${ns.name}/schemas`} />}
					>
						<ShieldCheck className="mr-1 h-3.5 w-3.5" />
						Schemas
					</Button>
				</div>
			</CardContent>
		</Card>
	);
}

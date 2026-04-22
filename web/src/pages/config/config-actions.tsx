import { Pencil } from "lucide-react";
import { LockAwareButton } from "@/components/lock-aware-button";
import { CopyDialog } from "./copy-dialog";
import { DeleteDialog } from "./delete-dialog";
import { LockDialog } from "./lock-dialog";

interface ConfigActionsProps {
	path: string;
	namespace: string;
	configLocked: boolean;
	namespaceLocked: boolean;
}

export function ConfigActions({
	path,
	namespace,
	configLocked,
	namespaceLocked,
}: ConfigActionsProps) {
	const effectiveLocked = configLocked || namespaceLocked;

	return (
		<div className="flex flex-wrap gap-2">
			<LockAwareButton
				variant="outline"
				size="sm"
				locked={effectiveLocked}
				to={`/config/edit/${namespace}${path}`}
			>
				<Pencil className="mr-1 h-4 w-4" />
				Edit
			</LockAwareButton>
			<CopyDialog sourcePath={path} sourceNamespace={namespace} />
			<LockDialog
				path={path}
				namespace={namespace}
				configLocked={configLocked}
				namespaceLocked={namespaceLocked}
			/>
			<DeleteDialog
				path={path}
				namespace={namespace}
				effectiveLocked={effectiveLocked}
			/>
		</div>
	);
}

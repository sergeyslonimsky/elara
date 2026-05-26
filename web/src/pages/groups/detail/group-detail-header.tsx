import { Shield } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import type { Group } from "@/gen/elara/group/v1/group_pb";

interface GroupDetailHeaderProps {
	group: Group;
}

export function GroupDetailHeader({ group }: Readonly<GroupDetailHeaderProps>) {
	return (
		<div className="flex items-start justify-between rounded-xl border bg-card p-4">
			<div className="flex flex-col gap-1">
				<div className="flex items-center gap-2">
					<h2 className="text-lg font-semibold">{group.name}</h2>
					{group.isSystem && (
						<Badge variant="secondary">
							<Shield className="mr-1 h-3 w-3" />
							System
						</Badge>
					)}
				</div>
				{group.description && (
					<p className="text-sm text-muted-foreground">{group.description}</p>
				)}
				<p className="font-mono text-xs text-muted-foreground">{group.id}</p>
			</div>
		</div>
	);
}

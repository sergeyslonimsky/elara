import { RefreshCw } from "lucide-react";
import type React from "react";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

interface PageHeaderProps {
	title: string;
	onRefresh?: () => void;
	isRefreshing?: boolean;
	children?: React.ReactNode;
}

export function PageHeader({
	title,
	onRefresh,
	isRefreshing,
	children,
}: PageHeaderProps) {
	return (
		<div className="flex h-14 shrink-0 items-center justify-between border-b px-4">
			<div className="flex items-center gap-3">
				<h1 className="font-semibold text-lg">{title}</h1>
				{onRefresh && (
					<Button
						variant="outline"
						size="icon-xs"
						onClick={onRefresh}
						aria-label="Refresh"
					>
						<RefreshCw
							className={cn("h-3.5 w-3.5", isRefreshing && "animate-spin")}
						/>
					</Button>
				)}
			</div>
			<div className="flex items-center gap-3">{children}</div>
		</div>
	);
}

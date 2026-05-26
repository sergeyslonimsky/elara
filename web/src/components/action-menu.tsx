import { MoreHorizontal } from "lucide-react";
import type { ReactNode } from "react";
import {
	DropdownMenu,
	DropdownMenuContent,
	DropdownMenuItem,
	DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";

interface ActionMenuItem {
	label: string;
	icon?: ReactNode;
	onClick: () => void;
	variant?: "default" | "destructive";
	disabled?: boolean;
}

interface ActionMenuProps {
	items: ActionMenuItem[];
	label?: string;
}

export function ActionMenu({
	items,
	label = "Open actions",
}: Readonly<ActionMenuProps>) {
	return (
		<DropdownMenu>
			<DropdownMenuTrigger
				aria-label={label}
				className="inline-flex h-7 w-7 items-center justify-center rounded-md p-0 hover:bg-muted focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
			>
				<MoreHorizontal className="h-4 w-4" />
			</DropdownMenuTrigger>
			<DropdownMenuContent align="end">
				{items.map((item) => (
					<DropdownMenuItem
						key={item.label}
						onClick={item.onClick}
						disabled={item.disabled}
						className={
							item.variant === "destructive" ? "text-destructive" : undefined
						}
					>
						{item.icon && <span className="mr-2">{item.icon}</span>}
						{item.label}
					</DropdownMenuItem>
				))}
			</DropdownMenuContent>
		</DropdownMenu>
	);
}

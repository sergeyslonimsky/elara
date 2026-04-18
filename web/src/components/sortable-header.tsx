import type { Column } from "@tanstack/react-table";
import { ArrowDown, ArrowUp, ArrowUpDown } from "lucide-react";
import { Button } from "@/components/ui/button";

interface SortableHeaderProps<T> {
	column: Column<T>;
	children: React.ReactNode;
}

export function SortableHeader<T>({
	column,
	children,
}: SortableHeaderProps<T>) {
	const sorted = column.getIsSorted();

	return (
		<Button
			variant="ghost"
			size="sm"
			className="-ml-3 h-8"
			onClick={() => column.toggleSorting(sorted === "asc")}
		>
			{children}
			{sorted === "asc" ? (
				<ArrowUp className="ml-1 h-3.5 w-3.5" />
			) : sorted === "desc" ? (
				<ArrowDown className="ml-1 h-3.5 w-3.5" />
			) : (
				<ArrowUpDown className="ml-1 h-3.5 w-3.5 text-muted-foreground" />
			)}
		</Button>
	);
}

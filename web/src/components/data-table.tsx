import {
	type ColumnDef,
	flexRender,
	getCoreRowModel,
	type SortingState,
	useReactTable,
} from "@tanstack/react-table";
import {
	Table,
	TableBody,
	TableCell,
	TableHead,
	TableHeader,
	TableRow,
} from "@/components/ui/table";
import { cn } from "@/lib/utils";

interface DataTableProps<TData, TValue> {
	columns: ColumnDef<TData, TValue>[];
	data: TData[];
	sorting?: SortingState;
	onSortingChange?: (sorting: SortingState) => void;
	onRowClick?: (row: TData, event: React.MouseEvent) => void;
	nameColumnWidth?: string;
	className?: string;
	hideBorder?: boolean;
}

export function DataTable<TData, TValue>({
	columns,
	data,
	sorting,
	onSortingChange,
	onRowClick,
	nameColumnWidth = "w-[50%]",
	className,
	hideBorder = false,
}: DataTableProps<TData, TValue>) {
	const table = useReactTable({
		data,
		columns,
		state: { sorting },
		onSortingChange: (updater) => {
			if (onSortingChange) {
				const next =
					typeof updater === "function" ? updater(sorting ?? []) : updater;
				onSortingChange(next);
			}
		},
		getCoreRowModel: getCoreRowModel(),
		manualSorting: true,
	});

	return (
		<div
			className={cn(
				!hideBorder && "rounded-xl border bg-card overflow-hidden",
				className,
			)}
		>
			<Table>
				<TableHeader>
					{table.getHeaderGroups().map((headerGroup) => (
						<TableRow key={headerGroup.id}>
							{headerGroup.headers.map((header) => (
								<TableHead
									key={header.id}
									className={header.id === "name" ? nameColumnWidth : ""}
								>
									{header.isPlaceholder
										? null
										: flexRender(
												header.column.columnDef.header,
												header.getContext(),
											)}
								</TableHead>
							))}
						</TableRow>
					))}
				</TableHeader>
				<TableBody>
					{table.getRowModel().rows.map((row) => (
						<TableRow
							key={row.id}
							className={cn(onRowClick && "cursor-pointer")}
							tabIndex={onRowClick ? 0 : undefined}
							onClick={(e) => onRowClick?.(row.original, e)}
							onKeyDown={(e) => {
								if (onRowClick && (e.key === "Enter" || e.key === " ")) {
									e.preventDefault();
									onRowClick(row.original, e as unknown as React.MouseEvent);
								}
							}}
						>
							{row.getVisibleCells().map((cell) => (
								<TableCell key={cell.id}>
									{flexRender(cell.column.columnDef.cell, cell.getContext())}
								</TableCell>
							))}
						</TableRow>
					))}
				</TableBody>
			</Table>
		</div>
	);
}

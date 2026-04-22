import { ChevronLeft, ChevronRight } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
	Select,
	SelectContent,
	SelectItem,
	SelectTrigger,
	SelectValue,
} from "@/components/ui/select";
import { PAGE_SIZE_OPTIONS } from "@/lib/constants";

interface PaginationControlsProps {
	total: number;
	pageSize: number;
	offset: number;
	onOffsetChange: (offset: number) => void;
	onPageSizeChange: (pageSize: number) => void;
}

export function PaginationControls({
	total,
	pageSize,
	offset,
	onOffsetChange,
	onPageSizeChange,
}: PaginationControlsProps) {
	if (total === 0) return null;

	const totalPages = Math.ceil(total / pageSize);
	const currentPage = Math.floor(offset / pageSize) + 1;
	const hasPrev = offset > 0;
	const hasNext = offset + pageSize < total;

	const startItem = offset + 1;
	const endItem = Math.min(offset + pageSize, total);

	// Generate page numbers to show (max 5 with ellipsis).
	function getPageNumbers(): (number | "...")[] {
		if (totalPages <= 7) {
			return Array.from({ length: totalPages }, (_, i) => i + 1);
		}

		const pages: (number | "...")[] = [1];

		if (currentPage > 3) pages.push("...");

		const start = Math.max(2, currentPage - 1);
		const end = Math.min(totalPages - 1, currentPage + 1);

		for (let i = start; i <= end; i++) {
			pages.push(i);
		}

		if (currentPage < totalPages - 2) pages.push("...");

		pages.push(totalPages);

		return pages;
	}

	return (
		<div className="flex items-center justify-between">
			<div className="flex items-center gap-2 text-muted-foreground text-sm">
				<span>
					{startItem}–{endItem} of {total}
				</span>
				<Select
					value={String(pageSize)}
					onValueChange={(v) => {
						if (!v) return;
						onPageSizeChange(Number(v));
					}}
				>
					<SelectTrigger className="h-8 w-[70px]">
						<SelectValue />
					</SelectTrigger>
					<SelectContent>
						{PAGE_SIZE_OPTIONS.map((size) => (
							<SelectItem key={size} value={String(size)}>
								{size}
							</SelectItem>
						))}
					</SelectContent>
				</Select>
				<span>per page</span>
			</div>

			{totalPages > 1 && (
				<div className="flex items-center gap-1">
					<Button
						variant="outline"
						size="icon-xs"
						disabled={!hasPrev}
						onClick={() => onOffsetChange(Math.max(0, offset - pageSize))}
					>
						<ChevronLeft className="h-4 w-4" />
					</Button>

					{getPageNumbers().map((page, i) =>
						page === "..." ? (
							<span
								// biome-ignore lint/suspicious/noArrayIndexKey: ellipsis placeholder
								key={`ellipsis-${i}`}
								className="px-1 text-muted-foreground text-sm"
							>
								...
							</span>
						) : (
							<Button
								key={page}
								variant={page === currentPage ? "default" : "outline"}
								size="icon-xs"
								onClick={() => onOffsetChange((page - 1) * pageSize)}
							>
								{page}
							</Button>
						),
					)}

					<Button
						variant="outline"
						size="icon-xs"
						disabled={!hasNext}
						onClick={() => onOffsetChange(offset + pageSize)}
					>
						<ChevronRight className="h-4 w-4" />
					</Button>
				</div>
			)}
		</div>
	);
}

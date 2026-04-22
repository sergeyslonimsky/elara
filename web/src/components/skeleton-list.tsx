import { Skeleton } from "@/components/ui/skeleton";
import { cn } from "@/lib/utils";

interface SkeletonListProps {
	count: number;
	/** Tailwind classes for each row (e.g. `"h-10 w-full"`). */
	className?: string;
	/** Wrapper classes (gap/spacing). Defaults to `"space-y-2"`. */
	wrapperClassName?: string;
}

/**
 * Renders `count` stacked Skeleton rows. Replaces ad-hoc
 * `Array.from({length}).map(_ => <Skeleton/>)` patterns used across list views.
 */
export function SkeletonList({
	count,
	className = "h-10 w-full",
	wrapperClassName = "space-y-2",
}: SkeletonListProps) {
	return (
		<div className={wrapperClassName}>
			{Array.from({ length: count }).map((_, i) => (
				<Skeleton
					// biome-ignore lint/suspicious/noArrayIndexKey: skeleton placeholder
					key={i}
					className={cn(className)}
				/>
			))}
		</div>
	);
}

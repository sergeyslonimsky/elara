import type { ReactNode } from "react";
import { PageHeader } from "@/components/page-header";

interface PageShellProps {
	title: string;
	onRefresh?: () => void;
	isRefreshing?: boolean;
	/** Right-aligned slot in the header (e.g. search input). */
	headerSlot?: ReactNode;
	children: ReactNode;
	/**
	 * Tailwind gap/padding for the content area. Defaults to `"flex flex-1 flex-col gap-4 p-4"`.
	 * Override only when a page needs different spacing (e.g. `gap-6`).
	 */
	contentClassName?: string;
}

/**
 * Unified page container — renders a `flex flex-col` outer, a `PageHeader`,
 * and a padded content area. Replaces the hand-rolled shell previously
 * inlined in every page component.
 */
export function PageShell({
	title,
	onRefresh,
	isRefreshing,
	headerSlot,
	children,
	contentClassName = "flex flex-1 flex-col gap-4 p-4",
}: Readonly<PageShellProps>) {
	return (
		<div className="flex flex-1 flex-col">
			<PageHeader
				title={title}
				onRefresh={onRefresh}
				isRefreshing={isRefreshing}
			>
				{headerSlot}
			</PageHeader>
			<div className={contentClassName}>{children}</div>
		</div>
	);
}

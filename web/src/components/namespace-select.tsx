import { useQuery } from "@connectrpc/connect-query";
import { Check, ChevronsUpDown, Loader2 } from "lucide-react";
import { useCallback, useEffect, useRef, useState } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
	Popover,
	PopoverContent,
	PopoverTrigger,
} from "@/components/ui/popover";
import { listNamespaces } from "@/gen/elara/namespace/v1/namespace_service-NamespaceService_connectquery";
import { cn } from "@/lib/utils";

const PAGE_SIZE = 20;

interface NamespaceSelectProps {
	value: string;
	onChange: (value: string) => void;
}

export function NamespaceSelect({ value, onChange }: NamespaceSelectProps) {
	const [open, setOpen] = useState(false);
	const [search, setSearch] = useState("");
	const [debouncedSearch, setDebouncedSearch] = useState("");
	const [limit, setLimit] = useState(PAGE_SIZE);
	const listRef = useRef<HTMLDivElement>(null);

	useEffect(() => {
		const timer = setTimeout(() => {
			setDebouncedSearch(search);
			setLimit(PAGE_SIZE);
		}, 200);
		return () => clearTimeout(timer);
	}, [search]);

	const { data, isFetching } = useQuery(listNamespaces, {
		pagination: { limit, offset: 0 },
		query: debouncedSearch || undefined,
	});

	const namespaces = data?.namespaces ?? [];
	const total = data?.pagination?.total ?? 0;
	const hasMore = namespaces.length < total;

	const handleScroll = useCallback(() => {
		const el = listRef.current;
		if (!el || isFetching || !hasMore) return;
		if (el.scrollTop + el.clientHeight >= el.scrollHeight - 20) {
			setLimit((prev) => prev + PAGE_SIZE);
		}
	}, [isFetching, hasMore]);

	return (
		<Popover open={open} onOpenChange={setOpen}>
			<PopoverTrigger
				render={
					<Button
						variant="outline"
						role="combobox"
						aria-expanded={open}
						className="w-full justify-between"
					/>
				}
			>
				{value || "Select namespace..."}
				<ChevronsUpDown className="ml-2 h-4 w-4 shrink-0 opacity-50" />
			</PopoverTrigger>
			<PopoverContent
				className="w-[--radix-popover-trigger-width] p-0"
				align="start"
			>
				<div className="border-b p-2">
					<Input
						placeholder="Search namespaces..."
						value={search}
						onChange={(e) => setSearch(e.target.value)}
						className="h-8"
					/>
				</div>
				<div
					ref={listRef}
					className="max-h-48 overflow-auto"
					onScroll={handleScroll}
				>
					{namespaces.length === 0 && !isFetching ? (
						<p className="px-3 py-4 text-center text-muted-foreground text-sm">
							{debouncedSearch ? "No namespaces found" : "No namespaces"}
						</p>
					) : (
						namespaces.map((ns) => (
							<button
								key={ns.name}
								type="button"
								className={cn(
									"flex w-full items-center gap-2 px-3 py-1.5 text-sm hover:bg-accent",
									value === ns.name && "bg-accent",
								)}
								onClick={() => {
									onChange(ns.name);
									setOpen(false);
								}}
							>
								<Check
									className={cn(
										"h-4 w-4 shrink-0",
										value === ns.name ? "opacity-100" : "opacity-0",
									)}
								/>
								{ns.name}
								<span className="ml-auto text-muted-foreground text-xs">
									{ns.configCount} configs
								</span>
							</button>
						))
					)}
					{isFetching && (
						<div className="flex justify-center py-2">
							<Loader2 className="h-4 w-4 animate-spin text-muted-foreground" />
						</div>
					)}
				</div>
			</PopoverContent>
		</Popover>
	);
}

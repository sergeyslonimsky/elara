import { useQuery } from "@connectrpc/connect-query";
import { Check, ChevronsUpDown, Loader2, X } from "lucide-react";
import { useCallback, useEffect, useRef, useState } from "react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
	Popover,
	PopoverContent,
	PopoverTrigger,
} from "@/components/ui/popover";
import { listNamespaces } from "@/gen/elara/namespace/v1/namespace_service-NamespaceService_connectquery";
import { useDebouncedValue } from "@/hooks/use-debounced-value";
import { cn } from "@/lib/utils";

const PAGE_SIZE = 20;

interface NamespaceMultiSelectProps {
	readonly value: readonly string[];
	readonly onChange: (value: string[]) => void;
	readonly placeholder?: string;
	readonly id?: string;
}

export function NamespaceMultiSelect({
	value,
	onChange,
	placeholder = "Select namespaces...",
	id,
}: NamespaceMultiSelectProps) {
	const [open, setOpen] = useState(false);
	const [search, setSearch] = useState("");
	const debouncedSearch = useDebouncedValue(search, 200);
	const [limit, setLimit] = useState(PAGE_SIZE);
	const listRef = useRef<HTMLDivElement>(null);

	// biome-ignore lint/correctness/useExhaustiveDependencies: trigger-only dep
	useEffect(() => {
		setLimit(PAGE_SIZE);
	}, [debouncedSearch]);

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

	const toggle = (name: string) => {
		if (value.includes(name)) {
			onChange(value.filter((v) => v !== name));
		} else {
			onChange([...value, name]);
		}
	};

	const remove = (name: string) => {
		onChange(value.filter((v) => v !== name));
	};

	return (
		<div className="flex flex-col gap-2">
			<Popover open={open} onOpenChange={setOpen}>
				<PopoverTrigger
					render={
						<Button
							id={id}
							type="button"
							variant="outline"
							role="combobox"
							aria-expanded={open}
							className="w-full justify-between font-normal"
						/>
					}
				>
					{value.length > 0
						? `${value.length} namespace${value.length === 1 ? "" : "s"} selected`
						: placeholder}
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
							namespaces.map((ns) => {
								const selected = value.includes(ns.name);
								return (
									<button
										key={ns.name}
										type="button"
										className={cn(
											"flex w-full items-center gap-2 px-3 py-1.5 text-sm hover:bg-accent",
											selected && "bg-accent",
										)}
										onClick={() => toggle(ns.name)}
									>
										<Check
											className={cn(
												"h-4 w-4 shrink-0",
												selected ? "opacity-100" : "opacity-0",
											)}
										/>
										{ns.name}
									</button>
								);
							})
						)}
						{isFetching && (
							<div className="flex justify-center py-2">
								<Loader2 className="h-4 w-4 animate-spin text-muted-foreground" />
							</div>
						)}
					</div>
				</PopoverContent>
			</Popover>

			{value.length > 0 && (
				<div className="flex flex-wrap gap-1">
					{value.map((name) => (
						<Badge key={name} variant="secondary" className="gap-1 pr-1">
							<span className="font-mono text-xs">{name}</span>
							<button
								type="button"
								onClick={() => remove(name)}
								className="rounded-sm hover:bg-muted-foreground/20"
								aria-label={`Remove ${name}`}
							>
								<X className="h-3 w-3" />
							</button>
						</Badge>
					))}
				</div>
			)}
		</div>
	);
}

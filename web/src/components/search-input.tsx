import { Search, X } from "lucide-react";
import { Input } from "@/components/ui/input";
import { cn } from "@/lib/utils";

interface SearchInputProps {
	value: string;
	onChange: (value: string) => void;
	onSearch?: () => void;
	onClear?: () => void;
	placeholder?: string;
	className?: string;
}

export function SearchInput({
	value,
	onChange,
	onSearch,
	onClear,
	placeholder = "Search...",
	className,
}: SearchInputProps) {
	return (
		<div className={cn("relative w-72", className)}>
			<Search className="absolute top-2 left-2.5 h-3.5 w-3.5 text-muted-foreground" />
			<Input
				placeholder={placeholder}
				className="h-8 pl-8 pr-8 text-sm"
				value={value}
				onChange={(e) => onChange(e.target.value)}
				onKeyDown={(e) => {
					if (e.key === "Enter") onSearch?.();
					if (e.key === "Escape" && value) onClear?.();
				}}
			/>
			{value && (
				<button
					type="button"
					className="absolute top-2 right-2.5 text-muted-foreground hover:text-foreground"
					onClick={onClear}
				>
					<X className="h-3.5 w-3.5" />
				</button>
			)}
		</div>
	);
}

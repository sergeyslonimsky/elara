import type { ReactNode } from "react";
import { Button } from "@/components/ui/button";
import {
	Combobox,
	ComboboxContent,
	ComboboxEmpty,
	ComboboxInput,
	ComboboxItem,
	ComboboxList,
	ComboboxTrigger,
	ComboboxValue,
} from "@/components/ui/combobox";
import type { Item } from "@/gen/elara/filter/v1/filter_pb";

export interface FilterComboboxProps {
	items: Item[];
	isLoading?: boolean;

	multiple?: boolean;
	value: string[];
	onValueChange: (value: string[]) => void;

	searchValue: string;
	onSearchChange: (query: string) => void;

	placeholder?: string;
	searchPlaceholder?: string;
	emptyMessage?: string;
	disabled?: boolean;
}

export function FilterCombobox({
	items,
	isLoading = false,
	multiple = true,
	value,
	onValueChange,
	searchValue,
	onSearchChange,
	placeholder = "Select...",
	searchPlaceholder = "Search...",
	emptyMessage = "Nothing found",
	disabled = false,
}: Readonly<FilterComboboxProps>) {
	const handleValueChange = (next: string | string[] | null) => {
		if (next === null) onValueChange([]);
		else if (Array.isArray(next)) onValueChange(next);
		else onValueChange([next]);
	};

	return (
		<Combobox
			items={items.map((it) => it.key)}
			multiple={multiple}
			value={multiple ? value : (value[0] ?? null)}
			onValueChange={handleValueChange}
			inputValue={searchValue}
			onInputValueChange={(v) => onSearchChange(v ?? "")}
			disabled={disabled}
		>
			<ComboboxTrigger
				render={
					<Button
						variant="outline"
						className="w-full justify-between font-normal"
					/>
				}
			>
				<ComboboxValue>
					{() => renderTriggerLabel(value, items, placeholder)}
				</ComboboxValue>
			</ComboboxTrigger>
			<ComboboxContent>
				<ComboboxInput placeholder={searchPlaceholder} showTrigger={false} />
				<ComboboxList>
					<ComboboxEmpty>
						{isLoading ? "Loading..." : emptyMessage}
					</ComboboxEmpty>
					{items.map((item) => (
						<ComboboxItem key={item.key} value={item.key}>
							{item.value}
						</ComboboxItem>
					))}
				</ComboboxList>
			</ComboboxContent>
		</Combobox>
	);
}

function renderTriggerLabel(
	value: string[],
	items: Item[],
	placeholder: string,
): ReactNode {
	if (value.length === 0) {
		return <span className="text-muted-foreground">{placeholder}</span>;
	}
	if (value.length === 1) {
		const matched = items.find((it) => it.key === value[0]);
		return matched ? matched.value : value[0];
	}
	return `${value.length} selected`;
}

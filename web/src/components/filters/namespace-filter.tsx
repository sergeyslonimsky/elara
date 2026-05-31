import { useQuery } from "@connectrpc/connect-query";
import { useState } from "react";
import { getNamespaces } from "@/gen/elara/filter/v1/filter_service-FilterService_connectquery";
import { useDebouncedValue } from "@/hooks/use-debounced-value";
import type { FilterProps } from "./types";
import { FilterCombobox } from "./ui/filter-combobox";

export function NamespaceFilter({
	value,
	onValueChange,
	permissionActions,
	multiple = true,
	disabled = false,
}: Readonly<FilterProps>) {
	const [search, setSearch] = useState("");
	const debouncedSearch = useDebouncedValue(search, 200);

	const { data, isLoading } = useQuery(getNamespaces, {
		filters: { query: debouncedSearch },
		actions: permissionActions ?? [],
	});

	return (
		<FilterCombobox
			items={data?.items ?? []}
			isLoading={isLoading}
			multiple={multiple}
			value={value}
			onValueChange={onValueChange}
			searchValue={search}
			onSearchChange={setSearch}
			placeholder="Select namespaces..."
			searchPlaceholder="Search namespaces..."
			emptyMessage="No namespaces found"
			disabled={disabled}
		/>
	);
}

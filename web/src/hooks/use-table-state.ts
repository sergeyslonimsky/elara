import type { SortingState } from "@tanstack/react-table";
import { useCallback, useMemo, useState } from "react";
import { sortingToParams } from "@/components/directory-table";
import { DEFAULT_PAGE_SIZE } from "@/lib/constants";

interface UseTableStateOptions {
	initialPageSize?: number;
	initialSorting?: SortingState;
}

export function useTableState(options: UseTableStateOptions = {}) {
	const [offset, setOffset] = useState(0);
	const [pageSize, setPageSize] = useState(
		options.initialPageSize ?? DEFAULT_PAGE_SIZE,
	);
	const [sorting, setSorting] = useState<SortingState>(
		options.initialSorting ?? [],
	);
	const [searchInput, setSearchInput] = useState("");
	const [query, setQuery] = useState("");

	const handleSearch = useCallback(() => {
		setQuery(searchInput);
		setOffset(0);
	}, [searchInput]);

	const handleClear = useCallback(() => {
		setSearchInput("");
		setQuery("");
		setOffset(0);
	}, []);

	const handleSortingChange = useCallback(
		(updaterOrValue: SortingState | ((old: SortingState) => SortingState)) => {
			setSorting(updaterOrValue);
			setOffset(0);
		},
		[],
	);

	const handlePageSizeChange = useCallback((size: number) => {
		setPageSize(size);
		setOffset(0);
	}, []);

	const reset = useCallback(() => {
		setOffset(0);
		setSorting(options.initialSorting ?? []);
		setSearchInput("");
		setQuery("");
	}, [options.initialSorting]);

	const sortParams = useMemo(() => sortingToParams(sorting), [sorting]);

	const paginationParams = useMemo(
		() => ({
			limit: pageSize,
			offset,
		}),
		[pageSize, offset],
	);

	return {
		// State
		offset,
		pageSize,
		sorting,
		searchInput,
		query,

		// Actions
		setOffset,
		setPageSize: handlePageSizeChange,
		setSorting: handleSortingChange,
		setSearchInput,
		handleSearch,
		handleClear,
		reset,

		// Formatted params for API
		sortParams,
		paginationParams,
	};
}

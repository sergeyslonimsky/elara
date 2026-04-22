import { useCallback, useState } from "react";

/**
 * Manage an Enter-to-search input pair: a draft (`searchInput`) and a
 * committed value (`query`) submitted either on Enter or via `handleSearch`.
 *
 * Use where full `useTableState` is overkill — e.g. the Namespaces page, which
 * isn't paginated server-side.
 */
export function useSearchInput() {
	const [searchInput, setSearchInput] = useState("");
	const [query, setQuery] = useState("");

	const handleSearch = useCallback(() => {
		setQuery(searchInput);
	}, [searchInput]);

	const handleClear = useCallback(() => {
		setSearchInput("");
		setQuery("");
	}, []);

	return { searchInput, setSearchInput, query, handleSearch, handleClear };
}

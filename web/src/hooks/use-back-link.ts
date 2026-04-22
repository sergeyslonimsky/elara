/**
 * Build the "back to parent" link for a config path.
 *
 *   /browse/${namespace}${parentPath}
 *
 * where `parentPath` is the path with the last segment stripped, falling back
 * to `"/"` for root configs. Used by ConfigPage and ConfigFormPage back buttons
 * and by DeleteDialog after deletion.
 */
export function useBackLink(namespace: string, path: string): string {
	const parentPath = path.split("/").slice(0, -1).join("/") || "/";
	return `/browse/${namespace}${parentPath}`;
}

/** Same as useBackLink, but returns only the parent path (no namespace prefix). */
export function parentPathOf(path: string): string {
	return path.split("/").slice(0, -1).join("/") || "/";
}

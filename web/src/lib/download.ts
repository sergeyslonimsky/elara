export function triggerDownload(
	data: Uint8Array,
	filename: string,
	contentType: string,
) {
	const blob = new Blob([data.buffer as ArrayBuffer], { type: contentType });
	const url = URL.createObjectURL(blob);
	const a = document.createElement("a");
	a.href = url;
	a.download = filename;
	a.click();
	URL.revokeObjectURL(url);
}

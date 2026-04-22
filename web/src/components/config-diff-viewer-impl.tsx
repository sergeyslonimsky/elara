import { DiffEditor } from "@monaco-editor/react";
import { useState } from "react";
import { useResolvedTheme } from "@/components/theme-provider";
import { Button } from "@/components/ui/button";

interface ConfigDiffViewerProps {
	original: string;
	modified: string;
	language?: string;
	height?: string;
	header?: React.ReactNode;
}

export function ConfigDiffViewer({
	original,
	modified,
	language = "plaintext",
	height = "400px",
	header,
}: Readonly<ConfigDiffViewerProps>) {
	const resolvedTheme = useResolvedTheme();

	const [sideBySide, setSideBySide] = useState(true);

	return (
		<div className="space-y-2">
			{header}
			<div className="flex justify-end">
				<Button
					variant="ghost"
					size="sm"
					onClick={() => setSideBySide((v) => !v)}
					className="text-xs"
				>
					{sideBySide ? "Inline" : "Side by side"}
				</Button>
			</div>
			<div className="overflow-hidden rounded-lg border">
				<DiffEditor
					height={height}
					language={language}
					theme={resolvedTheme === "dark" ? "vs-dark" : "vs"}
					original={original}
					modified={modified}
					options={{
						readOnly: true,
						renderSideBySide: sideBySide,
						minimap: { enabled: false },
						fontSize: 13,
						scrollBeyondLastLine: false,
						automaticLayout: true,
						padding: { top: 12 },
					}}
				/>
			</div>
		</div>
	);
}

import Editor from "@monaco-editor/react";
import { useResolvedTheme } from "@/components/theme-provider";

interface ConfigEditorProps {
	value: string;
	onChange?: (value: string) => void;
	language?: string;
	readOnly?: boolean;
	height?: string;
}

export function ConfigEditor({
	value,
	onChange,
	language = "plaintext",
	readOnly = false,
	height = "400px",
}: ConfigEditorProps) {
	const resolvedTheme = useResolvedTheme();

	return (
		<div className="overflow-hidden rounded-lg border">
			<Editor
				height={height}
				language={language}
				theme={resolvedTheme === "dark" ? "vs-dark" : "vs"}
				value={value}
				onChange={(v) => onChange?.(v ?? "")}
				options={{
					readOnly,
					minimap: { enabled: false },
					fontSize: 13,
					lineNumbers: "on",
					scrollBeyondLastLine: false,
					wordWrap: "on",
					tabSize: 2,
					automaticLayout: true,
					padding: { top: 12 },
				}}
			/>
		</div>
	);
}

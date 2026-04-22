import { useState } from "react";
import { Button } from "@/components/ui/button";
import {
	Card,
	CardContent,
	CardDescription,
	CardHeader,
	CardTitle,
} from "@/components/ui/card";
import { Input } from "@/components/ui/input";

interface MetadataEditorProps {
	metadata: Record<string, string>;
	onChange: (next: Record<string, string>) => void;
}

export function MetadataEditor({ metadata, onChange }: MetadataEditorProps) {
	const [metaKey, setMetaKey] = useState("");
	const [metaValue, setMetaValue] = useState("");

	const handleAdd = () => {
		if (metaKey && metaValue) {
			onChange({ ...metadata, [metaKey]: metaValue });
			setMetaKey("");
			setMetaValue("");
		}
	};

	const handleRemove = (key: string) => {
		const next = { ...metadata };
		delete next[key];
		onChange(next);
	};

	return (
		<Card className="rounded-xl">
			<CardHeader>
				<CardTitle className="text-sm">Metadata</CardTitle>
				<CardDescription>Optional key-value pairs</CardDescription>
			</CardHeader>
			<CardContent className="space-y-3">
				<div className="flex gap-2">
					<Input
						value={metaKey}
						onChange={(e) => setMetaKey(e.target.value)}
						placeholder="Key"
						className="flex-1"
					/>
					<Input
						value={metaValue}
						onChange={(e) => setMetaValue(e.target.value)}
						onKeyDown={(e) => {
							if (e.key === "Enter") {
								e.preventDefault();
								handleAdd();
							}
						}}
						placeholder="Value"
						className="flex-1"
					/>
					<Button
						type="button"
						variant="outline"
						size="sm"
						onClick={handleAdd}
						disabled={!metaKey || !metaValue}
					>
						Add
					</Button>
				</div>
				{Object.entries(metadata).map(([key, value]) => (
					<div
						key={key}
						className="flex items-center justify-between rounded-md bg-muted px-3 py-1.5 text-sm"
					>
						<span>
							<strong>{key}</strong>: {value}
						</span>
						<Button
							type="button"
							variant="ghost"
							size="xs"
							onClick={() => handleRemove(key)}
						>
							×
						</Button>
					</div>
				))}
				{Object.keys(metadata).length === 0 && (
					<p className="text-muted-foreground text-xs">No metadata added</p>
				)}
			</CardContent>
		</Card>
	);
}

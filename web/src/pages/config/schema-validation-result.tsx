interface SchemaViolation {
	path: string;
	message: string;
	keyword: string;
}

interface SchemaValidationResultProps {
	violations: SchemaViolation[];
}

export function SchemaValidationResult({
	violations,
}: Readonly<SchemaValidationResultProps>) {
	if (violations.length === 0) {
		return <p className="text-sm text-green-600">✓ Valid</p>;
	}

	return (
		<div className="space-y-1">
			{violations.map((v) => (
				<div
					key={`${v.path}-${v.keyword}-${v.message}`}
					className="text-sm text-destructive"
				>
					<span className="font-mono">{v.path || "/"}</span>: {v.message}
					{v.keyword && (
						<span className="ml-1 text-muted-foreground">[{v.keyword}]</span>
					)}
				</div>
			))}
		</div>
	);
}

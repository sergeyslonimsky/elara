import { Check, Copy, TriangleAlert } from "lucide-react";
import { useCallback, useEffect, useRef, useState } from "react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import {
	DialogDescription,
	DialogFooter,
	DialogHeader,
	DialogTitle,
} from "@/components/ui/dialog";
import { Field, FieldLabel } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { cn } from "@/lib/utils";

const ETCD_ENDPOINT_PLACEHOLDER = "<host>:2379";

export function buildEtcdctlExample(username: string, rawToken: string) {
	return `etcdctl --endpoints=${ETCD_ENDPOINT_PLACEHOLDER} --user ${username}:${rawToken} get <key>`;
}

interface CopyButtonProps {
	readonly value: string;
	readonly label: string;
	readonly className?: string;
}

function CopyButton({ value, label, className }: CopyButtonProps) {
	const [copied, setCopied] = useState(false);
	const timer = useRef<ReturnType<typeof setTimeout> | null>(null);

	useEffect(
		() => () => {
			if (timer.current) clearTimeout(timer.current);
		},
		[],
	);

	const handleCopy = useCallback(async () => {
		try {
			await navigator.clipboard.writeText(value);
			setCopied(true);
			if (timer.current) clearTimeout(timer.current);
			timer.current = setTimeout(() => setCopied(false), 2000);
		} catch {
			toast.error(`Failed to copy ${label.toLowerCase()}`);
		}
	}, [value, label]);

	return (
		<Button
			type="button"
			variant="outline"
			size="icon"
			onClick={handleCopy}
			aria-label={`Copy ${label.toLowerCase()}`}
			className={cn("shrink-0", className)}
		>
			{copied ? (
				<Check className="h-4 w-4 text-emerald-500" />
			) : (
				<Copy className="h-4 w-4" />
			)}
		</Button>
	);
}

interface CopyFieldProps {
	readonly id: string;
	readonly label: string;
	readonly value: string;
	readonly multiline?: boolean;
}

function CopyField({ id, label, value, multiline = false }: CopyFieldProps) {
	return (
		<Field>
			<FieldLabel htmlFor={id}>{label}</FieldLabel>
			<div className="flex items-start gap-2">
				{multiline ? (
					<Textarea
						id={id}
						readOnly
						value={value}
						rows={3}
						className="resize-none break-all font-mono text-xs"
						onFocus={(e) => e.currentTarget.select()}
					/>
				) : (
					<Input
						id={id}
						readOnly
						value={value}
						className="font-mono text-xs"
						onFocus={(e) => e.currentTarget.select()}
					/>
				)}
				<CopyButton value={value} label={label} />
			</div>
		</Field>
	);
}

interface TokenCreatedViewProps {
	readonly username: string;
	readonly rawToken: string;
	readonly onDone: () => void;
}

export function TokenCreatedView({
	username,
	rawToken,
	onDone,
}: TokenCreatedViewProps) {
	return (
		<>
			<DialogHeader>
				<DialogTitle>Token created</DialogTitle>
				<DialogDescription>
					Use the username and token below as etcd credentials.
				</DialogDescription>
			</DialogHeader>
			<div className="flex flex-col gap-4 py-2">
				<div
					role="alert"
					className="flex gap-2 rounded-md border border-amber-500/50 bg-amber-500/10 p-3 text-sm"
				>
					<TriangleAlert className="mt-0.5 h-4 w-4 shrink-0 text-amber-500" />
					<p>
						Copy the token now — it is shown only once and cannot be retrieved
						later. Store it in a secret manager.
					</p>
				</div>
				<CopyField id="tc-username" label="Username" value={username} />
				<CopyField id="tc-token" label="Token" value={rawToken} />
				<CopyField
					id="tc-example"
					label="Example"
					value={buildEtcdctlExample(username, rawToken)}
					multiline
				/>
			</div>
			<DialogFooter>
				<Button onClick={onDone}>Done</Button>
			</DialogFooter>
		</>
	);
}

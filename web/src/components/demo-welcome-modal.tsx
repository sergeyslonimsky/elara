import { useEffect, useState } from "react";
import { useAuth } from "@/components/auth-provider";
import { Button } from "@/components/ui/button";
import {
	Dialog,
	DialogContent,
	DialogDescription,
	DialogFooter,
	DialogHeader,
	DialogTitle,
} from "@/components/ui/dialog";

// Persist dismissal so the welcome modal only interrupts the first visit.
const DISMISSED_KEY = "elara.demo.welcome.dismissed";

// DemoWelcomeModal greets first-time visitors of an `elara:demo` instance. It
// renders only when the backend reports demo mode via the capabilities endpoint,
// and shows once per browser.
export function DemoWelcomeModal() {
	const { state } = useAuth();
	const demoMode =
		state.status === "authenticated" && state.capabilities.demoMode;

	const [open, setOpen] = useState(false);

	useEffect(() => {
		if (demoMode && localStorage.getItem(DISMISSED_KEY) !== "1") {
			setOpen(true);
		}
	}, [demoMode]);

	function handleOpenChange(next: boolean) {
		setOpen(next);
		if (!next) {
			localStorage.setItem(DISMISSED_KEY, "1");
		}
	}

	if (!demoMode) {
		return null;
	}

	return (
		<Dialog open={open} onOpenChange={handleOpenChange}>
			<DialogContent>
				<DialogHeader>
					<DialogTitle>Welcome to Elara</DialogTitle>
					<DialogDescription>
						This is a demo pre-loaded with sample data: three namespaces
						(production, staging, dev), ten config keys, and simulated
						Kubernetes clients reading them.
					</DialogDescription>
				</DialogHeader>
				<p className="text-sm text-muted-foreground">
					Try editing{" "}
					<code className="rounded bg-muted px-1 py-0.5 text-xs">
						production/api/limits.json
					</code>{" "}
					with an invalid value to see JSON Schema validation in action.
				</p>
				<DialogFooter>
					<Button onClick={() => handleOpenChange(false)}>
						Start exploring
					</Button>
				</DialogFooter>
			</DialogContent>
		</Dialog>
	);
}

import { FileQuestion } from "lucide-react";
import { Link } from "react-router";
import { buttonVariants } from "@/components/ui/button";

export function NotFoundPage() {
	return (
		<div className="flex flex-1 flex-col items-center justify-center gap-4">
			<FileQuestion className="h-16 w-16 text-muted-foreground" />
			<h1 className="font-semibold text-2xl">Page not found</h1>
			<p className="text-muted-foreground">
				The page you're looking for doesn't exist.
			</p>
			<Link to="/" className={buttonVariants()}>
				Go Home
			</Link>
		</div>
	);
}

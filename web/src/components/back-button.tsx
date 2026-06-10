import { ArrowLeft } from "lucide-react";
import { Link } from "react-router";
import { Button } from "@/components/ui/button";

interface BackButtonProps {
	to: string;
	label?: string;
	className?: string;
}

export function BackButton({
	to,
	label = "Back",
	className,
}: Readonly<BackButtonProps>) {
	return (
		<div className={className}>
			<Button variant="ghost" size="sm" render={<Link to={to} />}>
				<ArrowLeft className="mr-1 h-4 w-4" />
				{label}
			</Button>
		</div>
	);
}

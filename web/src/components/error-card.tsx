import { AlertCircle } from "lucide-react";
import { Card, CardContent } from "@/components/ui/card";

interface ErrorCardProps {
	message: string;
}

export function ErrorCard({ message }: ErrorCardProps) {
	return (
		<Card className="rounded-xl border-destructive">
			<CardContent className="flex items-center gap-3 pt-6">
				<AlertCircle className="h-5 w-5 text-destructive" />
				<p className="text-destructive">{message}</p>
			</CardContent>
		</Card>
	);
}

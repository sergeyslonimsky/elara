import { Sparkline } from "@/components/sparkline";
import { Card, CardContent } from "@/components/ui/card";
import { cn } from "@/lib/utils";

interface KpiCardProps {
	label: string;
	value: string | number;
	/** Optional subtitle below the main value (e.g. "since 2m ago"). */
	subtitle?: string;
	/** Optional time-series for the bottom sparkline. */
	series?: number[];
	/** Tailwind text color class for the sparkline (e.g. "text-emerald-500"). */
	accentClass?: string;
	className?: string;
}

export function KpiCard({
	label,
	value,
	subtitle,
	series,
	accentClass = "text-primary",
	className,
}: Readonly<KpiCardProps>) {
	return (
		<Card className={cn("rounded-xl", className)}>
			<CardContent className="space-y-2 pt-4">
				<div className="text-muted-foreground text-xs uppercase tracking-wide">
					{label}
				</div>
				<div className="font-semibold text-2xl tabular-nums">{value}</div>
				{subtitle && (
					<div className="text-muted-foreground text-xs">{subtitle}</div>
				)}
				{series && series.length > 1 && (
					<div className={cn("h-10", accentClass)}>
						<Sparkline data={series} />
					</div>
				)}
			</CardContent>
		</Card>
	);
}

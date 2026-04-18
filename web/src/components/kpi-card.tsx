import { useId } from "react";
import { Area, AreaChart, ResponsiveContainer } from "recharts";
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
}: KpiCardProps) {
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

function Sparkline({ data }: { data: number[] }) {
	const uid = useId();
	const series = data.map((v, i) => ({ i, v }));
	return (
		<ResponsiveContainer width="100%" height="100%">
			<AreaChart
				data={series}
				margin={{ top: 0, right: 0, bottom: 0, left: 0 }}
			>
				<defs>
					<linearGradient id={uid} x1="0" y1="0" x2="0" y2="1">
						<stop offset="0%" stopColor="currentColor" stopOpacity={0.4} />
						<stop offset="100%" stopColor="currentColor" stopOpacity={0} />
					</linearGradient>
				</defs>
				<Area
					type="monotone"
					dataKey="v"
					stroke="currentColor"
					strokeWidth={1.5}
					fill={`url(#${uid})`}
					isAnimationActive={false}
					dot={false}
				/>
			</AreaChart>
		</ResponsiveContainer>
	);
}

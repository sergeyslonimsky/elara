import { useId } from "react";
import { Area, AreaChart, ResponsiveContainer } from "recharts";

/**
 * Tiny time-series area chart used inside KpiCard. Extracted to its own file
 * so `React.lazy` (in sparkline.tsx) can defer recharts until a sparkline is
 * actually rendered — saves ~200KB on pages that don't show one.
 */
export function Sparkline({ data }: Readonly<{ data: number[] }>) {
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

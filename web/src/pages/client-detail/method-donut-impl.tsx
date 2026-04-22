import { BarChart3 } from "lucide-react";
import { useMemo } from "react";
import {
	Cell,
	Pie,
	PieChart,
	Tooltip as RechartsTooltip,
	ResponsiveContainer,
} from "recharts";
import {
	Empty,
	EmptyHeader,
	EmptyMedia,
	EmptyTitle,
} from "@/components/ui/empty";
import type { Client } from "@/gen/elara/clients/v1/clients_pb";

const DONUT_COLORS = [
	"hsl(217 91% 60%)",
	"hsl(142 71% 45%)",
	"hsl(38 92% 50%)",
	"hsl(280 65% 60%)",
	"hsl(0 84% 60%)",
	"hsl(240 5% 65%)", // "other"
];

function shortMethod(fullMethod: string): string {
	// "/etcdserverpb.KV/Put" → "KV/Put"
	const parts = fullMethod.split("/").filter(Boolean);
	if (parts.length < 2) return fullMethod;
	const svc = parts[0].split(".").pop() ?? parts[0];
	return `${svc}/${parts[1]}`;
}

export function MethodDonut({ client }: Readonly<{ client: Client }>) {
	const data = useMemo(() => {
		const entries = Object.entries(client.requestCounts ?? {})
			.map(([method, count]) => ({
				name: shortMethod(method),
				value: Number(count),
			}))
			.sort((a, b) => b.value - a.value);

		if (entries.length <= 5) return entries;

		const top = entries.slice(0, 5);
		const otherSum = entries.slice(5).reduce((sum, e) => sum + e.value, 0);
		return [...top, { name: "other", value: otherSum }];
	}, [client.requestCounts]);

	if (data.every((d) => d.value === 0)) {
		return (
			<Empty>
				<EmptyHeader>
					<EmptyMedia variant="icon">
						<BarChart3 />
					</EmptyMedia>
					<EmptyTitle>No data yet</EmptyTitle>
				</EmptyHeader>
			</Empty>
		);
	}

	return (
		<ResponsiveContainer width="100%" height="100%">
			<PieChart>
				<RechartsTooltip
					contentStyle={{
						borderRadius: 8,
						background: "hsl(var(--popover))",
						border: "1px solid hsl(var(--border))",
						fontSize: 12,
					}}
				/>
				<Pie
					data={data}
					dataKey="value"
					nameKey="name"
					innerRadius="55%"
					outerRadius="85%"
					paddingAngle={2}
					isAnimationActive={false}
					label={(entry) => entry.name}
					labelLine={false}
				>
					{data.map((entry, i) => (
						<Cell
							key={entry.name}
							fill={DONUT_COLORS[i % DONUT_COLORS.length]}
						/>
					))}
				</Pie>
			</PieChart>
		</ResponsiveContainer>
	);
}

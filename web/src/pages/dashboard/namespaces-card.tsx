import { FolderOpen } from "lucide-react";
import { Link } from "react-router";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import {
	Table,
	TableBody,
	TableCell,
	TableHead,
	TableHeader,
	TableRow,
} from "@/components/ui/table";
import type { Namespace } from "@/gen/elara/namespace/v1/namespace_pb";

const SKELETON_KEYS = ["n1", "n2", "n3", "n4"];

interface NamespacesCardProps {
	namespaces: Namespace[] | undefined;
	isLoading: boolean;
}

export function NamespacesCard({ namespaces, isLoading }: NamespacesCardProps) {
	return (
		<Card className="rounded-xl">
			<CardHeader className="pb-3">
				<CardTitle className="text-base">Namespaces</CardTitle>
			</CardHeader>
			<CardContent className="p-0">
				<Table>
					<TableHeader>
						<TableRow>
							<TableHead>Name</TableHead>
							<TableHead className="w-20 text-right">Configs</TableHead>
						</TableRow>
					</TableHeader>
					<TableBody>
						{isLoading &&
							SKELETON_KEYS.map((key) => (
								<TableRow key={key}>
									<TableCell colSpan={2}>
										<div className="h-4 animate-pulse rounded bg-muted" />
									</TableCell>
								</TableRow>
							))}
						{namespaces?.length === 0 && (
							<TableRow>
								<TableCell
									colSpan={2}
									className="py-8 text-center text-muted-foreground text-sm"
								>
									No namespaces
								</TableCell>
							</TableRow>
						)}
						{namespaces?.map((ns) => (
							<TableRow key={ns.name}>
								<TableCell>
									<Link
										to={`/browse/${ns.name}`}
										className="flex items-center gap-2 hover:underline"
									>
										<FolderOpen className="h-3.5 w-3.5 text-muted-foreground" />
										<span className="font-medium text-sm">{ns.name}</span>
									</Link>
								</TableCell>
								<TableCell className="text-right text-muted-foreground text-sm tabular-nums">
									{ns.configCount}
								</TableCell>
							</TableRow>
						))}
					</TableBody>
				</Table>
			</CardContent>
		</Card>
	);
}

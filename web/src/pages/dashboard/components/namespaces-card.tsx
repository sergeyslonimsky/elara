import { FolderOpen } from "lucide-react";
import { Link } from "react-router";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import {
	Table,
	TableBody,
	TableCell,
	TableHead,
	TableHeader,
	TableRow,
} from "@/components/ui/table";
import type { Namespace } from "@/gen/elara/namespace/v1/namespace_pb";

const SKELETON_ROWS = 4;

interface NamespacesCardProps {
	namespaces: Namespace[] | undefined;
	isLoading: boolean;
}

export function NamespacesCard({
	namespaces,
	isLoading,
}: Readonly<NamespacesCardProps>) {
	return (
		<Card className="rounded-xl">
			<CardHeader className="pb-3">
				<CardTitle className="text-base font-semibold">Namespaces</CardTitle>
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
							Array.from({ length: SKELETON_ROWS }).map((_, idx) => (
								// biome-ignore lint/suspicious/noArrayIndexKey: stable length
								<TableRow key={idx}>
									<TableCell>
										<div className="flex items-center gap-2">
											<Skeleton className="h-3.5 w-3.5" />
											<Skeleton className="h-4 w-24" />
										</div>
									</TableCell>
									<TableCell className="text-right">
										<Skeleton className="ml-auto h-4 w-8" />
									</TableCell>
								</TableRow>
							))}
						{!isLoading && namespaces?.length === 0 && (
							<TableRow>
								<TableCell
									colSpan={2}
									className="py-12 text-center text-muted-foreground text-sm"
								>
									No namespaces
								</TableCell>
							</TableRow>
						)}
						{!isLoading &&
							namespaces?.map((ns) => (
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

import { Skeleton } from "@/components/ui/skeleton";

export function FullScreenLoader() {
	return (
		<div
			className="flex h-screen w-screen items-center justify-center"
			role="status"
			aria-label="Loading"
		>
			<div className="space-y-4 text-center">
				<Skeleton className="mx-auto h-12 w-12 rounded-full" />
				<div className="space-y-2">
					<Skeleton className="h-4 w-[250px]" />
					<Skeleton className="h-4 w-[200px]" />
				</div>
			</div>
		</div>
	);
}

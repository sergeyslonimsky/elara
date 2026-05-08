import { Navigate, Outlet } from "react-router";
import { Skeleton } from "@/components/ui/skeleton";
import { useAuth } from "./auth-provider";

export function ProtectedRoute() {
	const { me, isLoading } = useAuth();

	if (isLoading) {
		return (
			<div className="flex h-screen w-screen items-center justify-center">
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

	if (!me) {
		return <Navigate to="/login" replace />;
	}

	return <Outlet />;
}

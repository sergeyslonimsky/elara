import { Navigate, Outlet, useLocation } from "react-router";
import { uiVisibility } from "@/auth/uiVisibility";
import { useAuth } from "@/components/auth-provider";
import { FullScreenError } from "@/components/full-screen-error";
import { FullScreenLoader } from "@/components/full-screen-loader";

const PUBLIC_PATHS = ["/login", "/auth/callback"];

export function AuthGuard() {
	const { state } = useAuth();
	const { pathname } = useLocation();

	if (state.status === "loading") {
		return <FullScreenLoader />;
	}

	if (state.status === "error") {
		return (
			<FullScreenError
				message={state.error.message || "Failed to connect to server"}
			/>
		);
	}

	if (state.status === "anonymous") {
		if (PUBLIC_PATHS.includes(pathname)) {
			return <Outlet />;
		}
		return <Navigate to="/login" replace />;
	}

	// authenticated
	const visibility = uiVisibility(state.ability);

	// Route protection based on visibility
	if (pathname.startsWith("/users") && !visibility.canSeeUsersSection) {
		return <Navigate to="/" replace />;
	}
	if (pathname.startsWith("/groups") && !visibility.canSeeGroupsSection) {
		return <Navigate to="/" replace />;
	}
	if (pathname.startsWith("/clients") && !visibility.canSeeClientsSection) {
		return <Navigate to="/" replace />;
	}
	if (
		pathname.startsWith("/namespaces") &&
		!visibility.canSeeNamespacesSection
	) {
		return <Navigate to="/" replace />;
	}
	if (pathname.startsWith("/browse") && !visibility.canSeeConfigsSection) {
		return <Navigate to="/" replace />;
	}

	if (state.user.passwordChangeRequired && pathname !== "/change-password") {
		return <Navigate to="/change-password" replace />;
	}

	// Not passwordChangeRequired: redirect away from auth pages
	if (
		!state.user.passwordChangeRequired &&
		(pathname === "/login" || pathname === "/change-password")
	) {
		return <Navigate to="/" replace />;
	}

	// passwordChangeRequired + pathname === "/change-password": allow through
	return <Outlet />;
}

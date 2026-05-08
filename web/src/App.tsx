import { Route, Routes } from "react-router";
import { AppLayout } from "@/components/app-layout";
import { ProtectedRoute } from "@/components/protected-route";
import { DashboardPage } from "@/pages/dashboard";
import { NotFoundPage } from "@/pages/not-found";
import { AuthRoutes } from "@/routes/auth-routes";
import { BrowseRoutes } from "@/routes/browse-routes";
import { ClientsRoutes } from "@/routes/clients-routes";
import { ConfigRoutes } from "@/routes/config-routes";
import { NamespacesRoutes } from "@/routes/namespaces-routes";
import { WebhooksRoutes } from "@/routes/webhooks-routes";

function App() {
	return (
		<Routes>
			{/* Public Routes */}
			{AuthRoutes}

			{/* Protected Routes */}
			<Route element={<ProtectedRoute />}>
				<Route element={<AppLayout />}>
					<Route path="/" element={<DashboardPage />} />

					{BrowseRoutes}
					{ConfigRoutes}
					{ClientsRoutes}
					{NamespacesRoutes}
					{WebhooksRoutes}

					{/* Admin/User Management Placeholders */}
					<Route path="/tokens" element={<div>Tokens Placeholder</div>} />
					<Route path="/users" element={<div>Users Placeholder</div>} />
					<Route path="/groups" element={<div>Groups Placeholder</div>} />
					<Route path="/access" element={<div>Access Placeholder</div>} />
				</Route>
			</Route>

			<Route path="*" element={<NotFoundPage />} />
		</Routes>
	);
}

export default App;

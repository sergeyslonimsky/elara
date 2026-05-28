import { Route, Routes } from "react-router";
import { AppLayout } from "@/components/app-layout";
import { AuthGuard } from "@/components/auth-guard";
import { DashboardPage } from "@/pages/dashboard";
import { NotFoundPage } from "@/pages/not-found";
import { AuthRoutes } from "@/routes/auth-routes";
import { BrowseRoutes } from "@/routes/browse-routes";
import { ClientsRoutes } from "@/routes/clients-routes";
import { ConfigRoutes } from "@/routes/config-routes";
import { NamespacesRoutes } from "@/routes/namespaces-routes";
import { TokensRoutes } from "@/routes/tokens-routes";
import { UsersRoutes } from "@/routes/users-routes";
import { WebhooksRoutes } from "@/routes/webhooks-routes";

function App() {
	return (
		<Routes>
			<Route element={<AuthGuard />}>
				{/* Auth Routes (login, change-password, callback) */}
				{AuthRoutes}

				{/* Protected Routes */}
				<Route element={<AppLayout />}>
					<Route path="/" element={<DashboardPage />} />

					{BrowseRoutes}
					{ConfigRoutes}
					{ClientsRoutes}
					{NamespacesRoutes}
					{WebhooksRoutes}
					{UsersRoutes}
					{TokensRoutes}

					{/* Admin/User Management Placeholders */}
					<Route path="/access" element={<div>Access Placeholder</div>} />
				</Route>

				<Route path="*" element={<NotFoundPage />} />
			</Route>
		</Routes>
	);
}

export default App;

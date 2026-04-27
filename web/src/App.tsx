import { Route, Routes } from "react-router";
import { AppLayout } from "@/components/app-layout";
import { DashboardPage } from "@/pages/dashboard";
import { NotFoundPage } from "@/pages/not-found";
import { BrowseRoutes } from "@/routes/browse-routes";
import { ClientsRoutes } from "@/routes/clients-routes";
import { ConfigRoutes } from "@/routes/config-routes";
import { NamespacesRoutes } from "@/routes/namespaces-routes";
import { WebhooksRoutes } from "@/routes/webhooks-routes";

function App() {
	return (
		<AppLayout>
			<Routes>
				<Route path="/" element={<DashboardPage />} />

				{BrowseRoutes}
				{ConfigRoutes}
				{ClientsRoutes}
				{NamespacesRoutes}
				{WebhooksRoutes}

				<Route path="*" element={<NotFoundPage />} />
			</Routes>
		</AppLayout>
	);
}

export default App;

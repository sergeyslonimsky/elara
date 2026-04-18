import { Navigate, Route, Routes } from "react-router";
import { AppLayout } from "@/components/app-layout";
import { BrowsePage } from "@/pages/browse";
import { ClientDetailPage } from "@/pages/client-detail";
import { ClientsPage } from "@/pages/clients";
import { ConfigPage } from "@/pages/config";
import { ConfigFormPage } from "@/pages/config-form";
import { DashboardPage } from "@/pages/dashboard";
import { NamespacesPage } from "@/pages/namespaces";
import { NotFoundPage } from "@/pages/not-found";

function App() {
	return (
		<AppLayout>
			<Routes>
				<Route path="/" element={<Navigate to="/dashboard" replace />} />
				<Route path="/dashboard" element={<DashboardPage />} />
				<Route path="/browse" element={<BrowsePage />} />
				<Route path="/browse/:namespace/*" element={<BrowsePage />} />
				<Route path="/config/new/:namespace/*" element={<ConfigFormPage />} />
				<Route path="/config/edit/:namespace/*" element={<ConfigFormPage />} />
				<Route path="/config/:namespace/*" element={<ConfigPage />} />
				<Route path="/namespaces" element={<NamespacesPage />} />
				<Route path="/clients" element={<ClientsPage />} />
				<Route path="/clients/:id" element={<ClientDetailPage />} />
				<Route path="*" element={<NotFoundPage />} />
			</Routes>
		</AppLayout>
	);
}

export default App;

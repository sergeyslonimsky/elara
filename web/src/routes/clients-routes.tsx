import { Route } from "react-router";
import { ClientDetailPage } from "@/pages/client-detail";
import { ClientsPage } from "@/pages/clients";

export const ClientsRoutes = (
	<Route path="clients">
		<Route index element={<ClientsPage />} />
		<Route path=":id" element={<ClientDetailPage />} />
	</Route>
);

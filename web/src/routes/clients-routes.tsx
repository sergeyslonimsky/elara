import { Route } from "react-router";
import { ClientsPage } from "@/pages/clients";
import { ClientDetailPage } from "@/pages/clients/detail";

export const ClientsRoutes = (
	<Route path="clients">
		<Route index element={<ClientsPage />} />
		<Route path=":id" element={<ClientDetailPage />} />
	</Route>
);

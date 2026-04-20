import { Route } from "react-router";
import { NamespacesPage } from "@/pages/namespaces";

export const NamespacesRoutes = (
	<Route path="namespaces">
		<Route index element={<NamespacesPage />} />
	</Route>
);

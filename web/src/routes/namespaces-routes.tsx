import { Route } from "react-router";
import { NamespacesPage } from "@/pages/namespaces";
import { SchemasPage } from "@/pages/namespaces/schemas-page";

export const NamespacesRoutes = (
	<Route path="namespaces">
		<Route index element={<NamespacesPage />} />
		<Route path=":name/schemas" element={<SchemasPage />} />
	</Route>
);

import { Route } from "react-router";
import { ConfigPage } from "@/pages/config";
import { ConfigFormPage } from "@/pages/config/form";

export const ConfigRoutes = (
	<Route path="config">
		<Route path="new/:namespace/*" element={<ConfigFormPage />} />
		<Route path="edit/:namespace/*" element={<ConfigFormPage />} />
		<Route path=":namespace/*" element={<ConfigPage />} />
	</Route>
);

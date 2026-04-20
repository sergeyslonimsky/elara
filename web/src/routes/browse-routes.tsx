import { Route } from "react-router";
import { BrowsePage } from "@/pages/browse";

export const BrowseRoutes = (
	<Route path="browse">
		<Route index element={<BrowsePage />} />
		<Route path=":namespace/*" element={<BrowsePage />} />
	</Route>
);

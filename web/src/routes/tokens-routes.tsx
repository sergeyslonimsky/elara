import { Route } from "react-router";
import { TokensPage } from "@/pages/tokens";

export const TokensRoutes = (
	<Route path="tokens">
		<Route index element={<TokensPage />} />
	</Route>
);

import { Route } from "react-router";
import { WebhooksPage } from "@/pages/webhooks";
import { WebhookHistoryPage } from "@/pages/webhooks/history";

export const WebhooksRoutes = (
	<Route path="webhooks">
		<Route index element={<WebhooksPage />} />
		<Route path=":id/history" element={<WebhookHistoryPage />} />
	</Route>
);

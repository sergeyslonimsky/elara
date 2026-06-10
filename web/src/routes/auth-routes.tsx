import { Route } from "react-router";
import { ChangePasswordPage } from "@/pages/change-password";
import { LoginPage } from "@/pages/login";
import { CallbackPage } from "@/pages/login/callback";

export const AuthRoutes = (
	<>
		<Route path="/login" element={<LoginPage />} />
		<Route path="/change-password" element={<ChangePasswordPage />} />
		<Route path="/auth/callback" element={<CallbackPage />} />
	</>
);

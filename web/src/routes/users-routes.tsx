import { Route } from "react-router";
import { GroupsPage } from "@/pages/groups";
import { UsersPage } from "@/pages/users";

export const UsersRoutes = (
	<>
		<Route path="users" element={<UsersPage />} />
		<Route path="groups" element={<GroupsPage />} />
	</>
);

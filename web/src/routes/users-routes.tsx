import { Route } from "react-router";
import { GroupsPage } from "@/pages/groups";
import { GroupDetailPage } from "@/pages/groups/detail";
import { UsersPage } from "@/pages/users";
import { UserDetailPage } from "@/pages/users/detail";

export const UsersRoutes = (
	<>
		<Route path="users">
			<Route index element={<UsersPage />} />
			<Route path=":email" element={<UserDetailPage />} />
		</Route>
		<Route path="groups">
			<Route index element={<GroupsPage />} />
			<Route path=":id" element={<GroupDetailPage />} />
		</Route>
	</>
);

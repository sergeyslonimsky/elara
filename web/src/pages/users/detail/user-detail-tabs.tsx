import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import type { User } from "@/gen/elara/user/v1/user_pb";
import { GroupsTab } from "./groups-tab";
import { ProfileTab } from "./profile-tab";

interface UserDetailTabsProps {
	user: User;
	visibleGroupIds: string[];
	membershipVersion: bigint;
}

export function UserDetailTabs({
	user,
	visibleGroupIds,
	membershipVersion,
}: Readonly<UserDetailTabsProps>) {
	return (
		<Tabs defaultValue="profile">
			<TabsList>
				<TabsTrigger value="profile">Profile</TabsTrigger>
				<TabsTrigger value="groups">Groups</TabsTrigger>
			</TabsList>
			<TabsContent value="profile">
				<ProfileTab user={user} />
			</TabsContent>
			<TabsContent value="groups">
				<GroupsTab
					user={user}
					visibleGroupIds={visibleGroupIds}
					membershipVersion={membershipVersion}
				/>
			</TabsContent>
		</Tabs>
	);
}

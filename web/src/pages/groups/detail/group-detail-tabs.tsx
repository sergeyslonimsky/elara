import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import type { PermissionAssignment } from "@/gen/elara/common/v1/permission_pb";
import type { Group } from "@/gen/elara/group/v1/group_pb";
import { MembersTab } from "./members-tab";
import { MetadataTab } from "./metadata-tab";
import { PermissionsTab } from "./permissions-tab";

interface GroupDetailTabsProps {
	group: Group;
	visibleMembers: string[];
	permissions: PermissionAssignment[];
}

export function GroupDetailTabs({
	group,
	visibleMembers,
	permissions,
}: Readonly<GroupDetailTabsProps>) {
	return (
		<Tabs defaultValue="metadata">
			<TabsList>
				<TabsTrigger value="metadata">Metadata</TabsTrigger>
				<TabsTrigger value="members">Members</TabsTrigger>
				<TabsTrigger value="permissions">Permissions</TabsTrigger>
			</TabsList>
			<TabsContent value="metadata">
				<MetadataTab group={group} />
			</TabsContent>
			<TabsContent value="members">
				<MembersTab group={group} visibleMembers={visibleMembers} />
			</TabsContent>
			<TabsContent value="permissions">
				<PermissionsTab group={group} permissions={permissions} />
			</TabsContent>
		</Tabs>
	);
}

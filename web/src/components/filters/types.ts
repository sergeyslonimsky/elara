import type { PermissionAction } from "@/gen/elara/common/v1/permission_pb";

export interface FilterProps {
	value: string[];
	onValueChange: (value: string[]) => void;
	permissionActions?: PermissionAction[];
	multiple?: boolean;
	disabled?: boolean;
}

import { timestampDate } from "@bufbuild/protobuf/wkt";
import { format } from "date-fns";
import type { User } from "@/gen/elara/user/v1/user_pb";

interface ProfileTabProps {
	user: User;
}

function Field({ label, value }: Readonly<{ label: string; value: string }>) {
	return (
		<>
			<dt className="text-sm font-medium text-muted-foreground">{label}</dt>
			<dd className="text-sm">{value}</dd>
		</>
	);
}

export function ProfileTab({ user }: Readonly<ProfileTabProps>) {
	const createdAt = user.createdAt
		? format(timestampDate(user.createdAt), "PPP p")
		: "-";
	const lastLoginAt = user.lastLoginAt
		? format(timestampDate(user.lastLoginAt), "PPP p")
		: "Never";

	return (
		<div className="rounded-xl border bg-card p-4 mt-2">
			<dl className="grid grid-cols-[12rem_1fr] gap-y-3">
				<Field label="Email" value={user.email} />
				<Field label="Name" value={user.name || "-"} />
				<Field label="Provider" value={user.provider || "internal"} />
				<Field label="Created at" value={createdAt} />
				<Field label="Last login" value={lastLoginAt} />
			</dl>
		</div>
	);
}

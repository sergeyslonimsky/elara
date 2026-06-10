import { Key, LogOut } from "lucide-react";
import { useNavigate } from "react-router";
import { useAuth } from "@/components/auth-provider";
import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar";
import {
	DropdownMenu,
	DropdownMenuContent,
	DropdownMenuGroup,
	DropdownMenuItem,
	DropdownMenuLabel,
	DropdownMenuSeparator,
	DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { AuthType } from "@/gen/elara/auth/v1/auth_pb";

export function UserMenu() {
	const { state, logout } = useAuth();
	const navigate = useNavigate();

	if (state.status !== "authenticated") return null;

	const { user: me, authType } = state;

	const initials = me.name
		? me.name
				.split(" ")
				.map((n) => n[0])
				.join("")
				.toUpperCase()
		: me.email[0].toUpperCase();

	return (
		<DropdownMenu>
			<DropdownMenuTrigger className="rounded-full p-1 hover:bg-muted cursor-pointer outline-none focus-visible:ring-2 focus-visible:ring-ring">
				<Avatar>
					{me.picture && <AvatarImage src={me.picture} alt={me.name} />}
					<AvatarFallback>{initials}</AvatarFallback>
				</Avatar>
			</DropdownMenuTrigger>
			<DropdownMenuContent align="end" className="w-56">
				<DropdownMenuGroup>
					<DropdownMenuLabel>
						<div className="flex flex-col space-y-1">
							<p className="text-sm font-medium leading-none">{me.name}</p>
							<p className="text-xs leading-none text-muted-foreground">
								{me.email}
							</p>
						</div>
					</DropdownMenuLabel>
				</DropdownMenuGroup>
				<DropdownMenuSeparator />
				{authType === AuthType.BASIC && (
					<DropdownMenuItem onClick={() => navigate("/change-password")}>
						<Key className="mr-2 h-4 w-4" />
						Change password
					</DropdownMenuItem>
				)}
				<DropdownMenuItem onClick={() => logout()}>
					<LogOut className="mr-2 h-4 w-4" />
					Sign out
				</DropdownMenuItem>
			</DropdownMenuContent>
		</DropdownMenu>
	);
}

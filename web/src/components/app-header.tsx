import { Monitor, Moon, Sun } from "lucide-react";
import { useParams } from "react-router";
import { useTheme } from "@/components/theme-provider";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
	DropdownMenu,
	DropdownMenuContent,
	DropdownMenuItem,
	DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Separator } from "@/components/ui/separator";
import { SidebarTrigger } from "@/components/ui/sidebar";

export function AppHeader() {
	const { namespace } = useParams();
	const { setTheme } = useTheme();

	return (
		<header className="flex h-14 shrink-0 items-center gap-3 border-b px-4">
			<SidebarTrigger className="-ml-1" />
			<Separator orientation="vertical" className="mr-1 h-4" />

			{namespace && (
				<Badge variant="outline" className="font-mono text-xs">
					{namespace}
				</Badge>
			)}

			<div className="ml-auto">
				<DropdownMenu>
					<DropdownMenuTrigger
						render={<Button variant="ghost" size="icon-xs" />}
					>
						<Sun className="h-4 w-4 scale-100 rotate-0 transition-all dark:scale-0 dark:-rotate-90" />
						<Moon className="absolute h-4 w-4 scale-0 rotate-90 transition-all dark:scale-100 dark:rotate-0" />
						<span className="sr-only">Toggle theme</span>
					</DropdownMenuTrigger>
					<DropdownMenuContent align="end">
						<DropdownMenuItem onClick={() => setTheme("light")}>
							<Sun className="mr-2 h-4 w-4" />
							Light
						</DropdownMenuItem>
						<DropdownMenuItem onClick={() => setTheme("dark")}>
							<Moon className="mr-2 h-4 w-4" />
							Dark
						</DropdownMenuItem>
						<DropdownMenuItem onClick={() => setTheme("system")}>
							<Monitor className="mr-2 h-4 w-4" />
							System
						</DropdownMenuItem>
					</DropdownMenuContent>
				</DropdownMenu>
			</div>
		</header>
	);
}

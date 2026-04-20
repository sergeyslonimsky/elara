import { AppHeader } from "@/components/app-header";
import { AppSidebar } from "@/components/app-sidebar";

interface AppLayoutProps {
	children: React.ReactNode;
}

export function AppLayout({ children }: AppLayoutProps) {
	return (
		<div className="flex min-h-screen">
			<AppSidebar />
			<div className="flex flex-1 flex-col">
				<AppHeader />
				<main className="flex-1 p-4">{children}</main>
			</div>
		</div>
	);
}

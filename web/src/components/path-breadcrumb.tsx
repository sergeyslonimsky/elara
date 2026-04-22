import { Home } from "lucide-react";
import { Link } from "react-router";
import {
	Breadcrumb,
	BreadcrumbItem,
	BreadcrumbLink,
	BreadcrumbList,
	BreadcrumbPage,
	BreadcrumbSeparator,
} from "@/components/ui/breadcrumb";

interface PathBreadcrumbProps {
	namespace?: string;
	path: string;
}

export function PathBreadcrumb({
	namespace,
	path,
}: Readonly<PathBreadcrumbProps>) {
	const pathSegments = path.split("/").filter(Boolean);

	// Build full breadcrumb segments: [namespace?, ...pathSegments]
	const segments: Array<{ label: string; href: string; isLast: boolean }> = [];

	if (namespace) {
		const isNamespaceLast = pathSegments.length === 0;
		segments.push({
			label: namespace,
			href: `/browse/${namespace}`,
			isLast: isNamespaceLast,
		});

		for (let i = 0; i < pathSegments.length; i++) {
			const segmentPath = `/${pathSegments.slice(0, i + 1).join("/")}`;
			segments.push({
				label: pathSegments[i],
				href: `/browse/${namespace}${segmentPath}`,
				isLast: i === pathSegments.length - 1,
			});
		}
	}

	return (
		<Breadcrumb>
			<BreadcrumbList>
				<BreadcrumbItem>
					<BreadcrumbLink aria-label="Root" render={<Link to="/browse" />}>
						<Home className="h-4 w-4" />
					</BreadcrumbLink>
				</BreadcrumbItem>

				{segments.map((seg) => (
					<span key={seg.href} className="contents">
						<BreadcrumbSeparator />
						<BreadcrumbItem>
							{seg.isLast ? (
								<BreadcrumbPage>{seg.label}</BreadcrumbPage>
							) : (
								<BreadcrumbLink render={<Link to={seg.href} />}>
									{seg.label}
								</BreadcrumbLink>
							)}
						</BreadcrumbItem>
					</span>
				))}
			</BreadcrumbList>
		</Breadcrumb>
	);
}

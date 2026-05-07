import { render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import { describe, expect, it } from "vitest";
import type { Namespace } from "@/gen/elara/namespace/v1/namespace_pb";
import { NamespacesCard } from "./namespaces-card";

describe("NamespacesCard", () => {
	it("renders loading state", () => {
		const { container } = render(
			<MemoryRouter>
				<NamespacesCard namespaces={undefined} isLoading={true} />
			</MemoryRouter>,
		);
		const skeletons = container.querySelectorAll('[data-slot="skeleton"]');
		expect(skeletons.length).toBeGreaterThan(0);
	});

	it("renders empty state", () => {
		render(
			<MemoryRouter>
				<NamespacesCard namespaces={[]} isLoading={false} />
			</MemoryRouter>,
		);
		expect(screen.getByText("No namespaces")).toBeInTheDocument();
	});

	it("renders namespaces", () => {
		const namespaces: Partial<Namespace>[] = [
			{
				name: "test-ns",
				configCount: 5,
			},
		];
		render(
			<MemoryRouter>
				<NamespacesCard
					namespaces={namespaces as Namespace[]}
					isLoading={false}
				/>
			</MemoryRouter>,
		);
		expect(screen.getByText("test-ns")).toBeInTheDocument();
		expect(screen.getByText("5")).toBeInTheDocument();
	});
});

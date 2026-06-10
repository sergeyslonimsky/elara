import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import type { Namespace } from "@/gen/elara/namespace/v1/namespace_pb";
import { TestProviders } from "@/test/test-utils";
import { NamespacesCard } from "./namespaces-card";

describe("NamespacesCard", () => {
	it("renders loading state", () => {
		const { container } = render(
			<TestProviders>
				<NamespacesCard namespaces={undefined} isLoading={true} />
			</TestProviders>,
		);
		const skeletons = container.querySelectorAll('[data-slot="skeleton"]');
		expect(skeletons.length).toBeGreaterThan(0);
	});

	it("renders empty state", () => {
		render(
			<TestProviders>
				<NamespacesCard namespaces={[]} isLoading={false} />
			</TestProviders>,
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
			<TestProviders>
				<NamespacesCard
					namespaces={namespaces as Namespace[]}
					isLoading={false}
				/>
			</TestProviders>,
		);
		expect(screen.getByText("test-ns")).toBeInTheDocument();
		expect(screen.getByText("5")).toBeInTheDocument();
	});
});

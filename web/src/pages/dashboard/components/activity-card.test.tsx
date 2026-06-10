import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { EventType } from "@/gen/elara/config/v1/config_pb";
import type { ActivityEntry } from "@/gen/elara/dashboard/v1/dashboard_service_pb";
import { TestProviders } from "@/test/test-utils";
import { ActivityCard } from "./activity-card";

describe("ActivityCard", () => {
	it("renders loading state", () => {
		const { container } = render(
			<TestProviders>
				<ActivityCard entries={undefined} isLoading={true} limit={20} />
			</TestProviders>,
		);
		// Check for skeletons using data-slot
		const skeletons = container.querySelectorAll('[data-slot="skeleton"]');
		expect(skeletons.length).toBeGreaterThan(0);
	});

	it("renders empty state", () => {
		render(
			<TestProviders>
				<ActivityCard entries={[]} isLoading={false} limit={20} />
			</TestProviders>,
		);
		expect(screen.getByText("No activity yet")).toBeInTheDocument();
	});

	it("renders entries", () => {
		const entries: Partial<ActivityEntry>[] = [
			{
				eventType: EventType.CREATED,
				namespace: "test-ns",
				path: "/test/path",
				revision: BigInt(1),
				timestamp: {
					seconds: BigInt(Math.floor(Date.now() / 1000)),
					nanos: 0,
				} as unknown as ActivityEntry["timestamp"],
			},
		];
		render(
			<TestProviders>
				<ActivityCard
					entries={entries as ActivityEntry[]}
					isLoading={false}
					limit={20}
				/>
			</TestProviders>,
		);
		expect(screen.getByText("test-ns")).toBeInTheDocument();
		expect(screen.getByText("/test/path")).toBeInTheDocument();
	});
});

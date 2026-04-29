import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { KpiCard } from "./kpi-card";

// Mock Sparkline to avoid recharts issues in this test
vi.mock("@/components/sparkline", () => ({
	Sparkline: () => <div data-testid="sparkline" />,
}));

describe("KpiCard", () => {
	it("renders label and value", () => {
		render(<KpiCard label="Total Requests" value="1,234" />);
		expect(screen.getByText("Total Requests")).toBeInTheDocument();
		expect(screen.getByText("1,234")).toBeInTheDocument();
	});

	it("renders subtitle when provided", () => {
		render(<KpiCard label="Label" value="100" subtitle="Since yesterday" />);
		expect(screen.getByText("Since yesterday")).toBeInTheDocument();
	});

	it("renders sparkline when series is provided", () => {
		render(<KpiCard label="Label" value="100" series={[1, 2, 3]} />);
		expect(screen.getByTestId("sparkline")).toBeInTheDocument();
	});

	it("does not render sparkline when series has 1 or fewer points", () => {
		render(<KpiCard label="Label" value="100" series={[1]} />);
		expect(screen.queryByTestId("sparkline")).not.toBeInTheDocument();
	});
});

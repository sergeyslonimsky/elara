import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { AppLayout } from "./app-layout";

vi.mock("@/components/app-header", () => ({
	AppHeader: () => <div data-testid="app-header" />,
}));

vi.mock("@/components/app-sidebar", () => ({
	AppSidebar: () => <div data-testid="app-sidebar" />,
}));

describe("AppLayout", () => {
	it("renders sidebar, header and children", () => {
		render(
			<AppLayout>
				<div data-testid="child">Child Content</div>
			</AppLayout>,
		);

		expect(screen.getByTestId("app-sidebar")).toBeInTheDocument();
		expect(screen.getByTestId("app-header")).toBeInTheDocument();
		expect(screen.getByTestId("child")).toBeInTheDocument();
	});
});

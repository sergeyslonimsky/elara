import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import {
	Empty,
	EmptyContent,
	EmptyDescription,
	EmptyHeader,
	EmptyMedia,
	EmptyTitle,
} from "./empty";

describe("Empty", () => {
	it("renders all sub-components correctly", () => {
		render(
			<Empty>
				<EmptyHeader>
					<EmptyMedia variant="icon" data-testid="media">
						<svg data-testid="icon" />
					</EmptyMedia>
					<EmptyTitle>No data found</EmptyTitle>
					<EmptyDescription>Try adjusting your filters.</EmptyDescription>
				</EmptyHeader>
				<EmptyContent>
					<button type="button">Clear filters</button>
				</EmptyContent>
			</Empty>,
		);

		expect(screen.getByText("No data found")).toBeInTheDocument();
		expect(screen.getByText("Try adjusting your filters.")).toBeInTheDocument();
		expect(screen.getByRole("button", { name: "Clear filters" })).toBeInTheDocument();
		expect(screen.getByTestId("icon")).toBeInTheDocument();
		expect(screen.getByTestId("media")).toHaveClass("bg-muted");
	});

	it("applies default variant to EmptyMedia", () => {
		render(<EmptyMedia data-testid="media" />);
		expect(screen.getByTestId("media")).toHaveClass("bg-transparent");
	});

	it("forwards className to components", () => {
		render(<Empty className="custom-empty" data-testid="empty-root" />);
		expect(screen.getByTestId("empty-root")).toHaveClass("custom-empty");
	});
});

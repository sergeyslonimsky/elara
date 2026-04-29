import { render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { ScrollArea } from "./scroll-area";

describe("ScrollArea", () => {
	it("renders correctly", async () => {
		render(
			<ScrollArea className="h-[200px] w-[350px]">
				<div style={{ height: "500px" }}>Long Content</div>
			</ScrollArea>,
		);

		await waitFor(() => {
			expect(screen.getByText("Long Content")).toBeInTheDocument();
		});
	});

	it("applies custom className", async () => {
		render(
			<ScrollArea className="custom-scroll-area">
				<div>Content</div>
			</ScrollArea>,
		);
		
		await waitFor(() => {
			expect(screen.getByText("Content").closest('[data-slot="scroll-area"]')).toHaveClass("custom-scroll-area");
		});
	});
});

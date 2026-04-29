import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from "./tooltip";

describe("Tooltip", () => {
	it("shows content on hover", async () => {
		const user = userEvent.setup();
		render(
			<TooltipProvider delay={0}>
				<Tooltip>
					<TooltipTrigger>Hover me</TooltipTrigger>
					<TooltipContent>Tooltip content</TooltipContent>
				</Tooltip>
			</TooltipProvider>
		);

		await user.hover(screen.getByText("Hover me"));
		
		// Base UI tooltips might have a small delay or use different states
		await waitFor(() => {
			expect(screen.getByText("Tooltip content")).toBeInTheDocument();
		});
	});
});

import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import {
	Popover,
	PopoverContent,
	PopoverDescription,
	PopoverHeader,
	PopoverTitle,
	PopoverTrigger,
} from "./popover";

describe("Popover", () => {
	it("opens and closes correctly", async () => {
		const user = userEvent.setup();
		render(
			<Popover>
				<PopoverTrigger>Open Popover</PopoverTrigger>
				<PopoverContent>
					<PopoverHeader>
						<PopoverTitle>Popover Title</PopoverTitle>
						<PopoverDescription>Popover Description</PopoverDescription>
					</PopoverHeader>
					<div>Content</div>
				</PopoverContent>
			</Popover>,
		);

		await user.click(screen.getByText("Open Popover"));
		expect(screen.getByText("Popover Title")).toBeInTheDocument();
		expect(screen.getByText("Popover Description")).toBeInTheDocument();

		// Popover typically closes when clicking outside or pressing Escape
		// Base UI might need specific focus management for tests
		await user.keyboard("{Escape}");
		await waitFor(() => {
			expect(screen.queryByText("Popover Title")).not.toBeInTheDocument();
		});
	});
});

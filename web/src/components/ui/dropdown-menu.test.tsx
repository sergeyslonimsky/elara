import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import {
	DropdownMenu,
	DropdownMenuCheckboxItem,
	DropdownMenuContent,
	DropdownMenuGroup,
	DropdownMenuItem,
	DropdownMenuLabel,
	DropdownMenuRadioGroup,
	DropdownMenuRadioItem,
	DropdownMenuSeparator,
	DropdownMenuTrigger,
} from "./dropdown-menu";

describe("DropdownMenu", () => {
	it("opens and shows items", async () => {
		const user = userEvent.setup();
		render(
			<DropdownMenu>
				<DropdownMenuTrigger>Open Menu</DropdownMenuTrigger>
				<DropdownMenuContent>
					<DropdownMenuGroup>
						<DropdownMenuLabel>Label</DropdownMenuLabel>
					</DropdownMenuGroup>
					<DropdownMenuSeparator />
					<DropdownMenuGroup>
						<DropdownMenuItem>Item 1</DropdownMenuItem>
						<DropdownMenuItem variant="destructive">Delete</DropdownMenuItem>
					</DropdownMenuGroup>
				</DropdownMenuContent>
			</DropdownMenu>,
		);

		await user.click(screen.getByText("Open Menu"));
		await waitFor(() => {
			expect(screen.getByText("Label")).toBeInTheDocument();
		});
		expect(screen.getByText("Item 1")).toBeInTheDocument();
		expect(screen.getByText("Delete")).toHaveAttribute("data-variant", "destructive");
	});

	it("handles checkbox and radio items", async () => {
		const user = userEvent.setup();
		render(
			<DropdownMenu open>
				<DropdownMenuContent>
					<DropdownMenuCheckboxItem checked>Checkbox</DropdownMenuCheckboxItem>
					<DropdownMenuRadioGroup value="2">
						<DropdownMenuRadioItem value="1">Radio 1</DropdownMenuRadioItem>
						<DropdownMenuRadioItem value="2">Radio 2</DropdownMenuRadioItem>
					</DropdownMenuRadioGroup>
				</DropdownMenuContent>
			</DropdownMenu>,
		);

		expect(screen.getByText("Checkbox")).toBeInTheDocument();
		expect(screen.getByText("Radio 1")).toBeInTheDocument();
		expect(screen.getByText("Radio 2")).toBeInTheDocument();
	});
});

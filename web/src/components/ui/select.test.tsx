import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import {
	Select,
	SelectContent,
	SelectItem,
	SelectTrigger,
	SelectValue,
} from "./select";

describe("Select", () => {
	it("opens and selects an item", async () => {
		const user = userEvent.setup();
		render(
			<Select defaultValue="apple">
				<SelectTrigger>
					<SelectValue placeholder="Select a fruit" />
				</SelectTrigger>
				<SelectContent>
					<SelectItem value="apple">Apple</SelectItem>
					<SelectItem value="banana">Banana</SelectItem>
				</SelectContent>
			</Select>,
		);

		const trigger = screen.getByRole("combobox");
		expect(trigger).toHaveTextContent(/apple/i);

		await user.click(trigger);
		const bananaItem = screen.getByText("Banana");
		await user.click(bananaItem);

		await waitFor(() => {
			expect(trigger).toHaveTextContent(/banana/i);
		});
	});
});

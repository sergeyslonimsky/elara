import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import {
	Combobox,
	ComboboxContent,
	ComboboxInput,
	ComboboxItem,
	ComboboxList,
} from "./combobox";

describe("Combobox", () => {
	it("allows selecting items", async () => {
		const user = userEvent.setup();
		render(
			<Combobox>
				<ComboboxInput placeholder="Search..." />
				<ComboboxContent>
					<ComboboxList>
						<ComboboxItem value="apple">Apple</ComboboxItem>
						<ComboboxItem value="banana">Banana</ComboboxItem>
					</ComboboxList>
				</ComboboxContent>
			</Combobox>,
		);

		const input = screen.getByPlaceholderText("Search...");
		await user.click(input);

		expect(screen.getByText("Apple")).toBeInTheDocument();
		expect(screen.getByText("Banana")).toBeInTheDocument();

		await user.click(screen.getByText("Banana"));
		
		await waitFor(() => {
			expect(input).toHaveValue("banana");
		});
	});
});

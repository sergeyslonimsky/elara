import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import { RadioGroup, RadioGroupItem } from "./radio-group";

describe("RadioGroup", () => {
	it("renders correctly and allows selection", async () => {
		const user = userEvent.setup();
		render(
			<RadioGroup defaultValue="option-1">
				<div className="flex items-center gap-2">
					<RadioGroupItem value="option-1" id="r1" />
					<label htmlFor="r1">Option 1</label>
				</div>
				<div className="flex items-center gap-2">
					<RadioGroupItem value="option-2" id="r2" />
					<label htmlFor="r2">Option 2</label>
				</div>
			</RadioGroup>,
		);

		const r1 = screen.getByRole("radio", { name: "Option 1" });
		const r2 = screen.getByRole("radio", { name: "Option 2" });

		expect(r1).toBeChecked();
		expect(r2).not.toBeChecked();

		await user.click(r2);
		expect(r1).not.toBeChecked();
		expect(r2).toBeChecked();
	});
});

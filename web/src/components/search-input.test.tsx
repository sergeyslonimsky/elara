import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { SearchInput } from "./search-input";

describe("SearchInput", () => {
	it("renders correctly with value", () => {
		render(<SearchInput value="test" onChange={() => {}} />);
		expect(screen.getByDisplayValue("test")).toBeInTheDocument();
	});

	it("calls onChange when typing", async () => {
		const onChange = vi.fn();
		const user = userEvent.setup();
		render(<SearchInput value="" onChange={onChange} />);

		const input = screen.getByPlaceholderText("Search...");
		await user.type(input, "a");
		expect(onChange).toHaveBeenCalledWith("a");
	});

	it("calls onSearch on Enter key", async () => {
		const onSearch = vi.fn();
		const user = userEvent.setup();
		render(
			<SearchInput value="test" onChange={() => {}} onSearch={onSearch} />,
		);

		const input = screen.getByDisplayValue("test");
		await user.type(input, "{Enter}");
		expect(onSearch).toHaveBeenCalledTimes(1);
	});

	it("calls onClear on Escape key", async () => {
		const onClear = vi.fn();
		const user = userEvent.setup();
		render(<SearchInput value="test" onChange={() => {}} onClear={onClear} />);

		const input = screen.getByDisplayValue("test");
		await user.type(input, "{Escape}");
		expect(onClear).toHaveBeenCalledTimes(1);
	});

	it("renders clear button when value is present and calls onClear", async () => {
		const onClear = vi.fn();
		const user = userEvent.setup();
		render(<SearchInput value="test" onChange={() => {}} onClear={onClear} />);

		const clearBtn = screen.getByRole("button");
		await user.click(clearBtn);
		expect(onClear).toHaveBeenCalledTimes(1);
	});
});

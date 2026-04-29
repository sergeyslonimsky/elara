import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { NamespaceSelect } from "./namespace-select";

// Mock useQuery
const mockUseQuery = vi.fn();
vi.mock("@connectrpc/connect-query", () => ({
	useQuery: (...args: unknown[]) => mockUseQuery(...args),
}));

describe("NamespaceSelect", () => {
	it("renders selected value and allows selecting from list", async () => {
		const onChange = vi.fn();
		const user = userEvent.setup();

		mockUseQuery.mockReturnValue({
			data: {
				namespaces: [
					{ name: "default", configCount: 5 },
					{ name: "production", configCount: 10 },
				],
				pagination: { total: 2 },
			},
			isFetching: false,
		});

		render(<NamespaceSelect value="default" onChange={onChange} />);

		const trigger = screen.getByRole("combobox");
		expect(trigger).toHaveTextContent("default");

		await user.click(trigger);

		const prodItem = screen.getByText("production");
		expect(prodItem).toBeInTheDocument();
		expect(screen.getByText("10 configs")).toBeInTheDocument();

		await user.click(prodItem);
		expect(onChange).toHaveBeenCalledWith("production");
	});

	it("shows empty message when no results found", async () => {
		const user = userEvent.setup();

		mockUseQuery.mockReturnValue({
			data: { namespaces: [], pagination: { total: 0 } },
			isFetching: false,
		});

		render(<NamespaceSelect value="" onChange={() => {}} />);

		await user.click(screen.getByRole("combobox"));
		expect(screen.getByText("No namespaces")).toBeInTheDocument();
	});
});

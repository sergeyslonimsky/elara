import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import { DeleteDialog } from "./delete-dialog";

const mockMutate = vi.fn();

vi.mock("@connectrpc/connect-query", () => ({
	useMutation: () => ({
		mutate: mockMutate,
		isPending: false,
	}),
}));

vi.mock("@tanstack/react-query", async (importOriginal) => {
	const actual = await importOriginal();
	return {
		...(actual as object),
		useQueryClient: () => ({ invalidateQueries: vi.fn() }),
	};
});

vi.mock("@/lib/queries", () => ({
	invalidate: vi.fn(),
}));

afterEach(() => {
	vi.clearAllMocks();
});

describe("DeleteDialog", () => {
	it("renders the trigger button", () => {
		render(
			<DeleteDialog webhookId="wh-1" webhookUrl="https://example.com/hook" />,
		);

		expect(
			screen.getByRole("button", { name: "Delete webhook" }),
		).toBeInTheDocument();
	});

	it("opens dialog showing webhookUrl after clicking trigger", async () => {
		const user = userEvent.setup();
		render(
			<DeleteDialog webhookId="wh-1" webhookUrl="https://example.com/hook" />,
		);

		await user.click(screen.getByRole("button", { name: "Delete webhook" }));

		expect(screen.getByText("Delete webhook?")).toBeInTheDocument();
		expect(screen.getByText("https://example.com/hook")).toBeInTheDocument();
	});

	it("calls mutate with correct id when Delete is clicked", async () => {
		const user = userEvent.setup();
		render(
			<DeleteDialog webhookId="wh-1" webhookUrl="https://example.com/hook" />,
		);

		await user.click(screen.getByRole("button", { name: "Delete webhook" }));
		await user.click(screen.getByRole("button", { name: "Delete" }));

		expect(mockMutate).toHaveBeenCalledWith({ id: "wh-1" });
	});

	it("closes dialog when Cancel is clicked", async () => {
		const user = userEvent.setup();
		render(
			<DeleteDialog webhookId="wh-1" webhookUrl="https://example.com/hook" />,
		);

		await user.click(screen.getByRole("button", { name: "Delete webhook" }));
		expect(screen.getByText("Delete webhook?")).toBeInTheDocument();

		await user.click(screen.getByRole("button", { name: "Cancel" }));

		expect(screen.queryByText("Delete webhook?")).not.toBeInTheDocument();
	});
});

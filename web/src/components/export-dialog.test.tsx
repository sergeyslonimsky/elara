import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { ExportDialog } from "./export-dialog";

// Mock useMutation
const mockMutate = vi.fn();
vi.mock("@connectrpc/connect-query", () => ({
	useMutation: () => ({
		mutate: mockMutate,
		isPending: false,
	}),
}));

describe("ExportDialog", () => {
	it("opens and triggers export mutation", async () => {
		const user = userEvent.setup();
		render(<ExportDialog namespace="test-ns" />);

		await user.click(screen.getByRole("button", { name: "Export" }));
		expect(screen.getByText("Export Namespace: test-ns")).toBeInTheDocument();

		// Change to YAML
		await user.click(screen.getByRole("radio", { name: "YAML" }));

		await user.click(screen.getByRole("button", { name: "Download" }));

		expect(mockMutate).toHaveBeenCalledWith(
			expect.objectContaining({
				namespace: "test-ns",
				encoding: 2, // YAML
				zip: false,
			}),
		);
	});

	it("shows ZIP layout options when Export All and ZIP is checked", async () => {
		const user = userEvent.setup();
		render(<ExportDialog />);

		await user.click(screen.getByRole("button", { name: "Export All" }));

		await user.click(screen.getByRole("checkbox", { name: "Compress as ZIP" }));

		expect(screen.getByText("ZIP layout")).toBeInTheDocument();
		expect(screen.getByText("One file per namespace")).toBeInTheDocument();
	});
});

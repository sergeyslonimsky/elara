import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ImportDialog } from "./import-dialog";

const mockMutate = vi.fn();

vi.mock("@connectrpc/connect-query", () => ({
	useMutation: () => ({
		mutate: mockMutate,
		isPending: false,
	}),
}));

describe("ImportDialog", () => {
	beforeEach(() => {
		class MockFileReader {
			onload: ((ev: ProgressEvent<FileReader>) => void) | null = null;
			readAsArrayBuffer = vi.fn().mockImplementation(() => {
				this.onload?.({
					target: { result: new ArrayBuffer(8) },
				} as ProgressEvent<FileReader>);
			});
		}
		vi.stubGlobal("FileReader", MockFileReader);
	});

	afterEach(() => {
		vi.unstubAllGlobals();
		vi.clearAllMocks();
	});

	it("opens and allows file selection and import", async () => {
		const user = userEvent.setup();
		render(<ImportDialog />);

		await user.click(screen.getByRole("button", { name: "Import" }));
		expect(screen.getByText("Import Configs")).toBeInTheDocument();

		const file = new File(['{"foo":"bar"}'], "config.json", {
			type: "application/json",
		});
		const input = screen.getByLabelText(/Upload file/i);

		await user.upload(input, file);

		expect(screen.getByText("Selected: config.json")).toBeInTheDocument();

		const previewBtn = screen.getByRole("button", { name: "Preview" });
		await user.click(previewBtn);

		expect(mockMutate).toHaveBeenCalledWith(
			expect.objectContaining({ dryRun: true }),
			expect.anything(),
		);

		const importBtn = screen.getByRole("button", { name: "Import" });
		await user.click(importBtn);

		expect(mockMutate).toHaveBeenCalledWith(
			expect.objectContaining({ dryRun: false }),
			expect.anything(),
		);
	});
});

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { ConfigDiffViewer } from "./config-diff-viewer";

// Mock DiffEditor from @monaco-editor/react
vi.mock("@monaco-editor/react", () => ({
	DiffEditor: ({
		original,
		modified,
	}: {
		original: string;
		modified: string;
	}) => (
		<div data-testid="diff-editor">
			<div data-testid="original">{original}</div>
			<div data-testid="modified">{modified}</div>
		</div>
	),
}));

describe("ConfigDiffViewer", () => {
	it("renders correctly", async () => {
		render(<ConfigDiffViewer original="old" modified="new" />);

		expect(await screen.findByTestId("original")).toHaveTextContent("old");
		expect(await screen.findByTestId("modified")).toHaveTextContent("new");
	});

	it("toggles side-by-side mode", async () => {
		const user = userEvent.setup();
		render(<ConfigDiffViewer original="old" modified="new" />);

		const toggleBtn = await screen.findByRole("button", { name: "Inline" });
		await user.click(toggleBtn);

		expect(
			screen.getByRole("button", { name: "Side by side" }),
		).toBeInTheDocument();
	});
});

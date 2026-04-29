import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { ConfigEditor } from "./config-editor";

// Mock Editor from @monaco-editor/react
vi.mock("@monaco-editor/react", () => ({
	default: ({ value }: { value: string }) => (
		<div data-testid="monaco-editor">{value}</div>
	),
}));

describe("ConfigEditor", () => {
	it("renders correctly", async () => {
		render(<ConfigEditor value="hello: world" language="yaml" />);

		// Since it's lazy, we might need to wait or just mock the impl too
		// But let's see if Suspense works out of the box in Vitest with happy-dom
		const editor = await screen.findByTestId("monaco-editor");
		expect(editor).toHaveTextContent("hello: world");
	});
});

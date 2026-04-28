import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import {
	Dialog,
	DialogContent,
	DialogDescription,
	DialogFooter,
	DialogHeader,
	DialogTitle,
	DialogTrigger,
} from "./dialog";

describe("Dialog", () => {
	it("opens and closes correctly", async () => {
		const user = userEvent.setup();
		render(
			<Dialog>
				<DialogTrigger>Open Dialog</DialogTrigger>
				<DialogContent>
					<DialogHeader>
						<DialogTitle>Dialog Title</DialogTitle>
						<DialogDescription>Dialog Description</DialogDescription>
					</DialogHeader>
					<div data-testid="dialog-body">Content</div>
					<DialogFooter showCloseButton>
						<button type="button">Action</button>
					</DialogFooter>
				</DialogContent>
			</Dialog>,
		);

		await user.click(screen.getByText("Open Dialog"));
		expect(screen.getByText("Dialog Title")).toBeInTheDocument();
		expect(screen.getByTestId("dialog-body")).toBeInTheDocument();

		await user.click(screen.getAllByText("Close")[0]);
		await waitFor(() => {
			expect(screen.queryByText("Dialog Title")).not.toBeInTheDocument();
		});
	});

	it("renders with different close button options", () => {
		render(
			<Dialog open>
				<DialogContent showCloseButton={false}>
					Title
				</DialogContent>
			</Dialog>
		);
		expect(screen.queryByText("Close")).not.toBeInTheDocument();
	});
});

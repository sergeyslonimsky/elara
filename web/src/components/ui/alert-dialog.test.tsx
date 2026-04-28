import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import {
	AlertDialog,
	AlertDialogAction,
	AlertDialogCancel,
	AlertDialogContent,
	AlertDialogDescription,
	AlertDialogFooter,
	AlertDialogHeader,
	AlertDialogTitle,
	AlertDialogTrigger,
} from "./alert-dialog";

describe("AlertDialog", () => {
	it("opens and closes correctly", async () => {
		const user = userEvent.setup();
		render(
			<AlertDialog>
				<AlertDialogTrigger>Open</AlertDialogTrigger>
				<AlertDialogContent>
					<AlertDialogHeader>
						<AlertDialogTitle>Are you sure?</AlertDialogTitle>
						<AlertDialogDescription>
							This action cannot be undone.
						</AlertDialogDescription>
					</AlertDialogHeader>
					<AlertDialogFooter>
						<AlertDialogCancel>Cancel</AlertDialogCancel>
						<AlertDialogAction>Continue</AlertDialogAction>
					</AlertDialogFooter>
				</AlertDialogContent>
			</AlertDialog>,
		);

		const trigger = screen.getByText("Open");
		await user.click(trigger);

		expect(screen.getByText("Are you sure?")).toBeInTheDocument();
		expect(screen.getByText("This action cannot be undone.")).toBeInTheDocument();

		const cancel = screen.getByText("Cancel");
		await user.click(cancel);

		await waitFor(() => {
			expect(screen.queryByText("Are you sure?")).not.toBeInTheDocument();
		});
	});

	it("renders media section if provided", async () => {
		const user = userEvent.setup();
		const { AlertDialogMedia } = await import("./alert-dialog");
		render(
			<AlertDialog open>
				<AlertDialogContent>
					<AlertDialogHeader>
						<AlertDialogMedia data-testid="alert-media">
							<svg />
						</AlertDialogMedia>
						<AlertDialogTitle>Title</AlertDialogTitle>
					</AlertDialogHeader>
				</AlertDialogContent>
			</AlertDialog>
		);

		expect(screen.getByTestId("alert-media")).toBeInTheDocument();
	});
});

import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import {
	Sheet,
	SheetContent,
	SheetDescription,
	SheetFooter,
	SheetHeader,
	SheetTitle,
	SheetTrigger,
} from "./sheet";

describe("Sheet", () => {
	it("opens and closes correctly", async () => {
		const user = userEvent.setup();
		render(
			<Sheet>
				<SheetTrigger>Open Sheet</SheetTrigger>
				<SheetContent side="right">
					<SheetHeader>
						<SheetTitle>Sheet Title</SheetTitle>
						<SheetDescription>Sheet Description</SheetDescription>
					</SheetHeader>
					<div>Content</div>
					<SheetFooter>Footer</SheetFooter>
				</SheetContent>
			</Sheet>,
		);

		await user.click(screen.getByText("Open Sheet"));
		expect(screen.getByText("Sheet Title")).toBeInTheDocument();

		await user.click(screen.getByText("Close"));
		await waitFor(() => {
			expect(screen.queryByText("Sheet Title")).not.toBeInTheDocument();
		});
	});
});

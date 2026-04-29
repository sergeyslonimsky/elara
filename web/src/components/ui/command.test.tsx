import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import {
	Command,
	CommandEmpty,
	CommandGroup,
	CommandInput,
	CommandItem,
	CommandList,
} from "./command";

describe("Command", () => {
	it("filters items based on input", async () => {
		const user = userEvent.setup();
		render(
			<Command>
				<CommandInput placeholder="Type a command..." />
				<CommandList>
					<CommandEmpty>No results found.</CommandEmpty>
					<CommandGroup heading="Suggestions">
						<CommandItem>Apple</CommandItem>
						<CommandItem>Banana</CommandItem>
					</CommandGroup>
				</CommandList>
			</Command>,
		);

		const input = screen.getByPlaceholderText("Type a command...");
		expect(screen.getByText("Apple")).toBeInTheDocument();
		expect(screen.getByText("Banana")).toBeInTheDocument();

		await user.type(input, "app");
		expect(screen.getByText("Apple")).toBeInTheDocument();
		expect(screen.queryByText("Banana")).not.toBeInTheDocument();
	});

	it("shows empty message when no results", async () => {
		const user = userEvent.setup();
		render(
			<Command>
				<CommandInput placeholder="Type a command..." />
				<CommandList>
					<CommandEmpty>No results found.</CommandEmpty>
					<CommandItem>Apple</CommandItem>
				</CommandList>
			</Command>,
		);

		const input = screen.getByPlaceholderText("Type a command...");
		await user.type(input, "xyz");
		expect(screen.getByText("No results found.")).toBeInTheDocument();
	});
});

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router";
import { describe, expect, it, vi } from "vitest";
import { LockAwareButton } from "./lock-aware-button";

describe("LockAwareButton", () => {
	it("renders disabled with title when locked", () => {
		render(
			<LockAwareButton locked lockedReason="It is locked">
				Action
			</LockAwareButton>,
		);

		const btn = screen.getByRole("button", { name: "Action" });
		expect(btn).toBeDisabled();
		expect(btn).toHaveAttribute("title", "It is locked");
	});

	it("renders as link when 'to' is provided and not locked", () => {
		render(
			<MemoryRouter>
				<LockAwareButton locked={false} to="/target">
					Go
				</LockAwareButton>
			</MemoryRouter>,
		);

		const link = screen.getByRole("link", { name: "Go" });
		expect(link).toHaveAttribute("href", "/target");
	});

	it("calls onClick when not locked and no 'to' provided", async () => {
		const onClick = vi.fn();
		const user = userEvent.setup();
		render(
			<LockAwareButton locked={false} onClick={onClick}>
				Click Me
			</LockAwareButton>,
		);

		await user.click(screen.getByRole("button", { name: "Click Me" }));
		expect(onClick).toHaveBeenCalledTimes(1);
	});
});

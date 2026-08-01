import { AbilityBuilder, createMongoAbility } from "@casl/ability";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { AppAbility } from "@/auth/ability";
import { authenticatedContext, TestProviders } from "@/test/test-utils";
import { DemoWelcomeModal } from "./demo-welcome-modal";

const DISMISSED_KEY = "elara.demo.welcome.dismissed";

function buildAbility(): AppAbility {
	const { build } = new AbilityBuilder<AppAbility>(createMongoAbility);
	return build();
}

// src/test/setup.ts replaces window.localStorage with a no-op stub (getItem
// always null, setItem a no-op) — spy on it instead of relying on real
// persistence.
describe("DemoWelcomeModal", () => {
	beforeEach(() => {
		vi.restoreAllMocks();
	});

	it("renders nothing when demo mode is off", () => {
		render(
			<TestProviders
				authContext={authenticatedContext(buildAbility(), {
					capabilities: { demoMode: false },
				})}
			>
				<DemoWelcomeModal />
			</TestProviders>,
		);

		expect(screen.queryByText("Welcome to Elara")).not.toBeInTheDocument();
	});

	it("shows the welcome dialog on first visit in demo mode", async () => {
		render(
			<TestProviders
				authContext={authenticatedContext(buildAbility(), {
					capabilities: { demoMode: true },
				})}
			>
				<DemoWelcomeModal />
			</TestProviders>,
		);

		await waitFor(() =>
			expect(screen.getByText("Welcome to Elara")).toBeInTheDocument(),
		);
	});

	it("does not reopen after the user dismisses it", () => {
		vi.spyOn(window.localStorage, "getItem").mockImplementation((key) =>
			key === DISMISSED_KEY ? "1" : null,
		);

		render(
			<TestProviders
				authContext={authenticatedContext(buildAbility(), {
					capabilities: { demoMode: true },
				})}
			>
				<DemoWelcomeModal />
			</TestProviders>,
		);

		expect(screen.queryByText("Welcome to Elara")).not.toBeInTheDocument();
	});

	it("persists dismissal to localStorage when closed", async () => {
		const setItemSpy = vi.spyOn(window.localStorage, "setItem");
		const user = userEvent.setup();

		render(
			<TestProviders
				authContext={authenticatedContext(buildAbility(), {
					capabilities: { demoMode: true },
				})}
			>
				<DemoWelcomeModal />
			</TestProviders>,
		);

		const button = await screen.findByRole("button", {
			name: "Start exploring",
		});
		await user.click(button);

		await waitFor(() =>
			expect(setItemSpy).toHaveBeenCalledWith(DISMISSED_KEY, "1"),
		);
	});
});

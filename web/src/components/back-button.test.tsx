import { render, screen } from "@testing-library/react";
import { describe, expect, test } from "vitest";
import { TestProviders } from "@/test/test-utils";
import { BackButton } from "./back-button";

describe("BackButton", () => {
	test("renders an anchor pointing at `to`", () => {
		render(
			<TestProviders>
				<BackButton to="/users" label="Back to users" />
			</TestProviders>,
		);
		const link = screen.getByRole("link", { name: /back to users/i });
		expect(link).toHaveAttribute("href", "/users");
	});

	test("uses default label when none provided", () => {
		render(
			<TestProviders>
				<BackButton to="/groups" />
			</TestProviders>,
		);
		expect(screen.getByRole("link", { name: /back/i })).toBeInTheDocument();
	});
});

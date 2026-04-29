import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { ErrorCard } from "./error-card";

describe("ErrorCard", () => {
	it("renders the error message", () => {
		render(<ErrorCard message="Something went wrong" />);
		expect(screen.getByText("Something went wrong")).toBeInTheDocument();
	});
});

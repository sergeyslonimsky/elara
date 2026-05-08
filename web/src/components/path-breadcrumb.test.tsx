import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { TestProviders } from "@/test/test-utils";
import { PathBreadcrumb } from "./path-breadcrumb";

describe("PathBreadcrumb", () => {
	it("renders correctly with namespace and path", () => {
		render(
			<TestProviders>
				<PathBreadcrumb namespace="my-ns" path="foo/bar" />
			</TestProviders>,
		);

		expect(screen.getByRole("link", { name: "Root" })).toBeInTheDocument();
		expect(screen.getByRole("link", { name: "my-ns" })).toBeInTheDocument();
		expect(screen.getByRole("link", { name: "foo" })).toBeInTheDocument();
		expect(screen.getByText("bar")).toBeInTheDocument(); // Last segment is BreadcrumbPage
	});

	it("renders only root when no namespace and path provided", () => {
		render(
			<TestProviders>
				<PathBreadcrumb path="" />
			</TestProviders>,
		);

		expect(screen.getByRole("link", { name: "Root" })).toBeInTheDocument();
		expect(
			screen.queryByRole("link", { name: "my-ns" }),
		).not.toBeInTheDocument();
	});

	it("handles deep paths correctly", () => {
		render(
			<TestProviders>
				<PathBreadcrumb namespace="ns" path="a/b/c/d" />
			</TestProviders>,
		);

		expect(screen.getByRole("link", { name: "a" })).toHaveAttribute(
			"href",
			"/browse/ns/a",
		);
		expect(screen.getByRole("link", { name: "b" })).toHaveAttribute(
			"href",
			"/browse/ns/a/b",
		);
		expect(screen.getByRole("link", { name: "c" })).toHaveAttribute(
			"href",
			"/browse/ns/a/b/c",
		);
		expect(screen.getByText("d")).toBeInTheDocument();
	});
});

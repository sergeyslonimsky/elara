import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { AuthType, type MeResponse } from "@/gen/elara/auth/v1/auth_service_pb";
import { TestProviders } from "@/test/test-utils";
import App from "./App";

// Mock useQuery to return authenticated state
vi.mock("@connectrpc/connect-query", async (importOriginal) => {
	const actual =
		await importOriginal<typeof import("@connectrpc/connect-query")>();
	return {
		...actual,
		useQuery: vi.fn(() => ({
			isLoading: false,
			isFetching: false,
			data: undefined,
			error: null,
		})),
	};
});

describe("App", () => {
	const authContext = {
		me: {
			name: "Anonymous",
			email: "anonymous@elara.local",
			isAdmin: true,
			namespaces: [],
			passwordChangeRequired: false,
			picture: "",
			canViewWebhooks: true,
			canManageWebhooks: true,
		} as unknown as MeResponse,
		authType: AuthType.NONE,
		isLoading: false,
		logout: vi.fn(),
	};

	it("renders the dashboard page at root route", async () => {
		render(
			<TestProviders initialEntries={["/"]} authContext={authContext}>
				<App />
			</TestProviders>,
		);

		expect(
			await screen.findByRole("heading", { name: "Dashboard" }),
		).toBeInTheDocument();
	});

	it("renders the 404 page for unknown routes", () => {
		render(
			<TestProviders
				initialEntries={["/unknown-route"]}
				authContext={authContext}
			>
				<App />
			</TestProviders>,
		);

		expect(screen.getByText("Page not found")).toBeInTheDocument();
		expect(
			screen.getByText("The page you're looking for doesn't exist."),
		).toBeInTheDocument();
	});

	it("renders navigation sidebar", () => {
		render(
			<TestProviders initialEntries={["/"]} authContext={authContext}>
				<App />
			</TestProviders>,
		);

		expect(
			screen.getAllByRole("button", { name: "Toggle Sidebar" }),
		).toHaveLength(2);
	});

	it("renders app header", () => {
		render(
			<TestProviders initialEntries={["/"]} authContext={authContext}>
				<App />
			</TestProviders>,
		);

		expect(screen.getByRole("banner")).toBeInTheDocument();
	});
});

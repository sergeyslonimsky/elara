import { create } from "@bufbuild/protobuf";
import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { buildAbility } from "@/auth/ability";
import type { AuthContextType } from "@/components/auth-provider";
import { AuthType } from "@/gen/elara/auth/v1/auth_pb";
import { GetCapabilitiesResponseSchema } from "@/gen/elara/capabilities/v1/capabilities_service_pb";
import { MeResponseSchema } from "@/gen/elara/profile/v1/profile_service_pb";
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
	const authContext: AuthContextType = {
		state: {
			status: "authenticated",
			authType: AuthType.NONE,
			user: create(MeResponseSchema, {
				name: "Anonymous",
				email: "anonymous@elara.local",
				permissions: [],
				passwordChangeRequired: false,
				picture: "",
			}),
			ability: buildAbility([{ object: 99, action: 99, domain: "*" } as any]),
			capabilities: create(GetCapabilitiesResponseSchema, {
				etcdTokenAuthEnabled: true,
				userManagementEnabled: true,
			}),
		},
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

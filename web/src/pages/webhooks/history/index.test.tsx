import { render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import { afterEach, describe, expect, it, vi } from "vitest";
import { WebhookHistoryPage } from "./index";

const mockUseQuery = vi.fn();

vi.mock("@connectrpc/connect-query", () => ({
	useQuery: (...args: unknown[]) => mockUseQuery(...args),
}));

vi.mock("react-router", async (importOriginal) => {
	const actual = await importOriginal();
	return {
		...(actual as object),
		useParams: () => ({ id: "wh-1" }),
	};
});

afterEach(() => {
	vi.clearAllMocks();
});

describe("WebhookHistoryPage", () => {
	it("renders the Delivery History heading", () => {
		mockUseQuery.mockReturnValue({
			data: undefined,
			isLoading: true,
			error: null,
		});

		render(
			<MemoryRouter>
				<WebhookHistoryPage />
			</MemoryRouter>,
		);

		expect(
			screen.getByRole("heading", { name: "Delivery History" }),
		).toBeInTheDocument();
	});

	it("shows Back to webhooks button", () => {
		mockUseQuery.mockReturnValue({
			data: undefined,
			isLoading: true,
			error: null,
		});

		render(
			<MemoryRouter>
				<WebhookHistoryPage />
			</MemoryRouter>,
		);

		expect(
			screen.getByRole("link", { name: /Back to webhooks/i }),
		).toBeInTheDocument();
	});

	it("shows webhook URL when webhook data is loaded", () => {
		mockUseQuery.mockImplementation((schema: unknown) => {
			if ((schema as { name?: string })?.name === "GetWebhook") {
				return {
					data: { webhook: { id: "wh-1", url: "https://example.com/hook" } },
					isLoading: false,
					error: null,
				};
			}
			return { data: { attempts: [] }, isLoading: false, error: null };
		});

		render(
			<MemoryRouter>
				<WebhookHistoryPage />
			</MemoryRouter>,
		);

		expect(screen.getByText("https://example.com/hook")).toBeInTheDocument();
	});

	it("shows empty state when attempts is empty", () => {
		mockUseQuery.mockReturnValue({
			data: { webhook: undefined, attempts: [] },
			isLoading: false,
			error: null,
		});

		render(
			<MemoryRouter>
				<WebhookHistoryPage />
			</MemoryRouter>,
		);

		expect(screen.getByText("No delivery attempts yet")).toBeInTheDocument();
	});

	it("shows attempt rows when attempts are loaded", () => {
		mockUseQuery.mockImplementation((schema: unknown) => {
			if ((schema as { name?: string })?.name === "GetWebhook") {
				return {
					data: { webhook: { id: "wh-1", url: "https://example.com/hook" } },
					isLoading: false,
					error: null,
				};
			}
			return {
				data: {
					attempts: [
						{
							attemptNumber: 1,
							success: true,
							statusCode: 200,
							latencyMs: BigInt(42),
							error: "",
							timestamp: null,
						},
					],
				},
				isLoading: false,
				error: null,
			};
		});

		render(
			<MemoryRouter>
				<WebhookHistoryPage />
			</MemoryRouter>,
		);

		expect(screen.getByText("1")).toBeInTheDocument();
		expect(screen.getByText("200")).toBeInTheDocument();
		expect(screen.getByText("42ms")).toBeInTheDocument();
	});

	it("shows error card when query errors", () => {
		mockUseQuery.mockReturnValue({
			data: undefined,
			isLoading: false,
			error: { message: "Failed to fetch webhook" },
		});

		render(
			<MemoryRouter>
				<WebhookHistoryPage />
			</MemoryRouter>,
		);

		expect(screen.getByText("Failed to fetch webhook")).toBeInTheDocument();
	});
});

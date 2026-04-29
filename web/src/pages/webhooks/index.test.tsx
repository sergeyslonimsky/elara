import { render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import { afterEach, describe, expect, it, vi } from "vitest";
import { WebhooksPage } from "./index";

const mockUseQuery = vi.fn();
const mockUseMutation = vi.fn();

vi.mock("@connectrpc/connect-query", () => ({
	useQuery: (...args: unknown[]) => mockUseQuery(...args),
	useMutation: (...args: unknown[]) => mockUseMutation(...args),
}));

vi.mock("@tanstack/react-query", async (importOriginal) => {
	const actual = await importOriginal();
	return {
		...(actual as object),
		useQueryClient: () => ({ invalidateQueries: vi.fn() }),
	};
});

vi.mock("@/lib/queries", () => ({
	invalidate: vi.fn(),
}));

vi.mock("./webhook-sheet", () => ({
	WebhookSheet: () => null,
}));

vi.mock("./delete-dialog", () => ({
	DeleteDialog: () => null,
}));

afterEach(() => {
	vi.clearAllMocks();
});

function defaultMutationMock() {
	mockUseMutation.mockReturnValue({ mutate: vi.fn(), isPending: false });
}

describe("WebhooksPage", () => {
	it("renders the Webhooks heading", () => {
		mockUseQuery.mockReturnValue({
			data: undefined,
			isLoading: true,
			error: null,
			refetch: vi.fn(),
			isFetching: false,
		});
		defaultMutationMock();

		render(
			<MemoryRouter>
				<WebhooksPage />
			</MemoryRouter>,
		);

		expect(
			screen.getByRole("heading", { name: "Webhooks" }),
		).toBeInTheDocument();
	});

	it("shows Add Webhook button", () => {
		mockUseQuery.mockReturnValue({
			data: undefined,
			isLoading: true,
			error: null,
			refetch: vi.fn(),
			isFetching: false,
		});
		defaultMutationMock();

		render(
			<MemoryRouter>
				<WebhooksPage />
			</MemoryRouter>,
		);

		expect(
			screen.getByRole("button", { name: /Add Webhook/i }),
		).toBeInTheDocument();
	});

	it("shows empty state when data has no webhooks", () => {
		mockUseQuery.mockReturnValue({
			data: { webhooks: [] },
			isLoading: false,
			error: null,
			refetch: vi.fn(),
			isFetching: false,
		});
		defaultMutationMock();

		render(
			<MemoryRouter>
				<WebhooksPage />
			</MemoryRouter>,
		);

		expect(screen.getByText("No webhooks")).toBeInTheDocument();
	});

	it("shows webhook URLs when data is returned", () => {
		mockUseQuery.mockReturnValue({
			data: {
				webhooks: [
					{
						id: "wh-1",
						url: "https://example.com/hook",
						namespaceFilter: "",
						pathPrefix: "",
						events: [1],
						enabled: true,
					},
				],
			},
			isLoading: false,
			error: null,
			refetch: vi.fn(),
			isFetching: false,
		});
		defaultMutationMock();

		render(
			<MemoryRouter>
				<WebhooksPage />
			</MemoryRouter>,
		);

		expect(screen.getByText("https://example.com/hook")).toBeInTheDocument();
	});

	it("shows ErrorCard when query errors", () => {
		mockUseQuery.mockReturnValue({
			data: undefined,
			isLoading: false,
			error: { message: "Network failure" },
			refetch: vi.fn(),
			isFetching: false,
		});
		defaultMutationMock();

		render(
			<MemoryRouter>
				<WebhooksPage />
			</MemoryRouter>,
		);

		expect(screen.getByText("Network failure")).toBeInTheDocument();
	});
});

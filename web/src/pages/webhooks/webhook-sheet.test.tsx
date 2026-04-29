import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { Webhook } from "@/gen/elara/webhook/v1/webhook_pb";
import { WebhookEvent } from "@/gen/elara/webhook/v1/webhook_pb";
import { WebhookSheet } from "./webhook-sheet";

const mockCreateMutate = vi.fn();
const mockUpdateMutate = vi.fn();

vi.mock("@connectrpc/connect-query", () => ({
	useMutation: (
		schema: { name?: string; typeName?: string } & Record<string, unknown>,
	) => {
		const name = schema?.typeName ?? schema?.name ?? "";
		if (String(name).toLowerCase().includes("create")) {
			return { mutate: mockCreateMutate, isPending: false };
		}
		return { mutate: mockUpdateMutate, isPending: false };
	},
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

afterEach(() => {
	vi.clearAllMocks();
});

const existingWebhook: Webhook = {
	id: "wh-1",
	url: "https://existing.com/hook",
	namespaceFilter: "production",
	pathPrefix: "/api/",
	events: [WebhookEvent.CREATED, WebhookEvent.UPDATED],
	enabled: true,
} as Webhook;

describe("WebhookSheet", () => {
	it("shows Add Webhook title in create mode", () => {
		render(<WebhookSheet open={true} onOpenChange={vi.fn()} />);

		expect(screen.getByText("Add Webhook")).toBeInTheDocument();
	});

	it("shows Edit Webhook title in edit mode", () => {
		render(
			<WebhookSheet
				open={true}
				onOpenChange={vi.fn()}
				webhook={existingWebhook}
			/>,
		);

		expect(screen.getByText("Edit Webhook")).toBeInTheDocument();
	});

	it("pre-fills URL field in edit mode", () => {
		render(
			<WebhookSheet
				open={true}
				onOpenChange={vi.fn()}
				webhook={existingWebhook}
			/>,
		);

		expect(
			screen.getByPlaceholderText("https://example.com/webhook"),
		).toHaveValue("https://existing.com/hook");
	});

	it("submit button is disabled when URL is empty", async () => {
		const user = userEvent.setup();
		render(<WebhookSheet open={true} onOpenChange={vi.fn()} />);

		const urlInput = screen.getByPlaceholderText("https://example.com/webhook");
		await user.clear(urlInput);

		expect(
			screen.getByRole("button", { name: "Create webhook" }),
		).toBeDisabled();
	});

	it("submit button is enabled when URL is filled and events selected", async () => {
		const user = userEvent.setup();
		render(<WebhookSheet open={true} onOpenChange={vi.fn()} />);

		const urlInput = screen.getByPlaceholderText("https://example.com/webhook");
		await user.type(urlInput, "https://new.example.com/hook");

		expect(
			screen.getByRole("button", { name: "Create webhook" }),
		).not.toBeDisabled();
	});

	it("calls createMutate on submit in create mode", async () => {
		const user = userEvent.setup();
		render(<WebhookSheet open={true} onOpenChange={vi.fn()} />);

		const urlInput = screen.getByPlaceholderText("https://example.com/webhook");
		await user.type(urlInput, "https://new.example.com/hook");

		await user.click(screen.getByRole("button", { name: "Create webhook" }));

		expect(mockCreateMutate).toHaveBeenCalledWith(
			expect.objectContaining({ url: "https://new.example.com/hook" }),
		);
	});

	it("calls updateMutate on submit in edit mode", async () => {
		const user = userEvent.setup();
		render(
			<WebhookSheet
				open={true}
				onOpenChange={vi.fn()}
				webhook={existingWebhook}
			/>,
		);

		await user.click(screen.getByRole("button", { name: "Save changes" }));

		expect(mockUpdateMutate).toHaveBeenCalledWith(
			expect.objectContaining({ id: "wh-1", url: "https://existing.com/hook" }),
		);
	});
});

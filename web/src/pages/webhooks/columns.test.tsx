import type { ColumnDef } from "@tanstack/react-table";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { Webhook } from "@/gen/elara/webhook/v1/webhook_pb";
import { WebhookEvent } from "@/gen/elara/webhook/v1/webhook_pb";
import { makeColumns } from "./columns";

vi.mock("./delete-dialog", () => ({
	DeleteDialog: () => null,
}));

afterEach(() => {
	vi.clearAllMocks();
});

function makeWebhook(overrides: Partial<Webhook> = {}): Webhook {
	return {
		id: "wh-1",
		url: "https://example.com/hook",
		namespaceFilter: "",
		pathPrefix: "",
		events: [WebhookEvent.CREATED, WebhookEvent.UPDATED],
		enabled: true,
		...overrides,
	} as Webhook;
}

function renderCell<T>(column: ColumnDef<T>, row: T) {
	if (typeof column.cell !== "function")
		throw new Error("cell is not a function");
	const cellContext = {
		row: { original: row },
		getValue: () => undefined,
		renderValue: () => undefined,
		cell: {} as never,
		column: {} as never,
		table: {} as never,
	};
	const result = column.cell(cellContext as never);
	return render(<MemoryRouter>{result as React.ReactElement}</MemoryRouter>);
}

describe("makeColumns", () => {
	it("url column renders webhook url", () => {
		const onEdit = vi.fn();
		const onToggleEnabled = vi.fn();
		const columns = makeColumns({ onEdit, onToggleEnabled });
		const urlColumn = columns[0];
		const webhook = makeWebhook();

		renderCell(urlColumn, webhook);

		expect(screen.getByText("https://example.com/hook")).toBeInTheDocument();
	});

	it("events column renders event badge labels", () => {
		const onEdit = vi.fn();
		const onToggleEnabled = vi.fn();
		const columns = makeColumns({ onEdit, onToggleEnabled });
		const eventsColumn = columns[1];
		const webhook = makeWebhook({
			events: [WebhookEvent.CREATED, WebhookEvent.DELETED],
		});

		renderCell(eventsColumn, webhook);

		expect(screen.getByText("created")).toBeInTheDocument();
		expect(screen.getByText("deleted")).toBeInTheDocument();
	});

	it("filters column shows All when no filters set", () => {
		const onEdit = vi.fn();
		const onToggleEnabled = vi.fn();
		const columns = makeColumns({ onEdit, onToggleEnabled });
		const filtersColumn = columns[2];
		const webhook = makeWebhook({ namespaceFilter: "", pathPrefix: "" });

		renderCell(filtersColumn, webhook);

		expect(screen.getByText("All")).toBeInTheDocument();
	});

	it("filters column shows namespace and prefix when set", () => {
		const onEdit = vi.fn();
		const onToggleEnabled = vi.fn();
		const columns = makeColumns({ onEdit, onToggleEnabled });
		const filtersColumn = columns[2];
		const webhook = makeWebhook({
			namespaceFilter: "production",
			pathPrefix: "/services/",
		});

		renderCell(filtersColumn, webhook);

		expect(screen.getByText("production")).toBeInTheDocument();
		expect(screen.getByText("/services/")).toBeInTheDocument();
	});

	it("enabled column renders checked checkbox when enabled", () => {
		const onEdit = vi.fn();
		const onToggleEnabled = vi.fn();
		const columns = makeColumns({ onEdit, onToggleEnabled });
		const enabledColumn = columns[3];
		const webhook = makeWebhook({ enabled: true });

		renderCell(enabledColumn, webhook);

		expect(screen.getByRole("checkbox")).toBeChecked();
	});

	it("enabled column renders unchecked checkbox when disabled", () => {
		const onEdit = vi.fn();
		const onToggleEnabled = vi.fn();
		const columns = makeColumns({ onEdit, onToggleEnabled });
		const enabledColumn = columns[3];
		const webhook = makeWebhook({ enabled: false });

		renderCell(enabledColumn, webhook);

		expect(screen.getByRole("checkbox")).not.toBeChecked();
	});

	it("enabled column calls onToggleEnabled when checkbox changes", async () => {
		const user = userEvent.setup();
		const onEdit = vi.fn();
		const onToggleEnabled = vi.fn();
		const columns = makeColumns({ onEdit, onToggleEnabled });
		const enabledColumn = columns[3];
		const webhook = makeWebhook({ enabled: true });

		renderCell(enabledColumn, webhook);

		await user.click(screen.getByRole("checkbox"));

		expect(onToggleEnabled).toHaveBeenCalledWith(webhook, false);
	});

	it("actions column has edit button that calls onEdit", async () => {
		const user = userEvent.setup();
		const onEdit = vi.fn();
		const onToggleEnabled = vi.fn();
		const columns = makeColumns({ onEdit, onToggleEnabled });
		const actionsColumn = columns[4];
		const webhook = makeWebhook();

		renderCell(actionsColumn, webhook);

		await user.click(screen.getByRole("button", { name: "Edit webhook" }));

		expect(onEdit).toHaveBeenCalledWith(webhook);
	});

	it("actions column has delivery history link", () => {
		const onEdit = vi.fn();
		const onToggleEnabled = vi.fn();
		const columns = makeColumns({ onEdit, onToggleEnabled });
		const actionsColumn = columns[4];
		const webhook = makeWebhook();

		renderCell(actionsColumn, webhook);

		expect(
			screen.getByRole("link", { name: "Delivery history" }),
		).toHaveAttribute("href", "/webhooks/wh-1/history");
	});
});

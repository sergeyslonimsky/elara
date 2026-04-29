import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, useNavigate } from "react-router";
import { describe, expect, it, vi } from "vitest";
import type { Client } from "@/gen/elara/clients/v1/clients_pb";
import { ClientsTable } from "./clients-table";

vi.mock("react-router", async (importOriginal) => {
	const actual = await importOriginal<typeof import("react-router")>();
	return {
		...actual,
		useNavigate: vi.fn(),
	};
});

describe("ClientsTable", () => {
	const mockClients: Partial<Client>[] = [
		{
			id: "1",
			clientName: "Client A",
			clientVersion: "1.0",
			connectedAt: {
				seconds: BigInt(Math.floor(Date.now() / 1000) - 3600),
				nanos: 0,
			} as never,
			lastActivityAt: {
				seconds: BigInt(Math.floor(Date.now() / 1000) - 10),
				nanos: 0,
			} as never,
			activeWatches: 1,
			errorCount: 0n,
			k8sPod: "pod-a",
		},
	];

	it("renders active clients", () => {
		render(
			<MemoryRouter>
				<ClientsTable
					clients={mockClients as Client[]}
					isLoading={false}
					mode="active"
					sorting={[]}
					onSortingChange={() => {}}
				/>
			</MemoryRouter>,
		);

		expect(screen.getByText("Client A")).toBeInTheDocument();
		expect(screen.getByText("v1.0")).toBeInTheDocument();
		expect(screen.getByText("pod-a")).toBeInTheDocument();
	});

	it("renders loading state", () => {
		const { container } = render(
			<MemoryRouter>
				<ClientsTable
					clients={[]}
					isLoading={true}
					mode="active"
					sorting={[]}
					onSortingChange={() => {}}
				/>
			</MemoryRouter>,
		);

		expect(
			container.querySelectorAll('[data-slot="skeleton"]').length,
		).toBeGreaterThan(0);
	});

	it("renders empty slot when no clients", () => {
		render(
			<MemoryRouter>
				<ClientsTable
					clients={[]}
					isLoading={false}
					mode="active"
					sorting={[]}
					onSortingChange={() => {}}
					emptySlot={<div data-testid="empty">No clients</div>}
				/>
			</MemoryRouter>,
		);

		expect(screen.getByTestId("empty")).toBeInTheDocument();
	});

	it("navigates on row click", async () => {
		const navigate = vi.fn();
		vi.mocked(useNavigate).mockReturnValue(navigate);
		const user = userEvent.setup();

		render(
			<MemoryRouter>
				<ClientsTable
					clients={mockClients as Client[]}
					isLoading={false}
					mode="active"
					sorting={[]}
					onSortingChange={() => {}}
				/>
			</MemoryRouter>,
		);

		await user.click(screen.getByText("Client A"));
		expect(navigate).toHaveBeenCalledWith("/clients/1");
	});
});

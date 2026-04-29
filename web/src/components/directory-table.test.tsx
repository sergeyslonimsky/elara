import { create } from "@bufbuild/protobuf";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, useNavigate } from "react-router";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { DirectoryEntrySchema } from "@/gen/elara/config/v1/config_pb";
import { DirectoryTable } from "./directory-table";

vi.mock("react-router", async (importOriginal) => {
	const actual = await importOriginal<typeof import("react-router")>();
	return {
		...actual,
		useNavigate: vi.fn(),
	};
});

describe("DirectoryTable", () => {
	beforeEach(() => {
		vi.clearAllMocks();
	});

	const mockEntries = [
		create(DirectoryEntrySchema, {
			name: "my-config",
			isFile: true,
			fullPath: "/my-config",
			format: 1, // JSON
			version: 1n,
			revision: 0n,
			locked: false,
			namespaceLocked: false,
			childCount: 0,
		}),
		create(DirectoryEntrySchema, {
			name: "subfolder",
			isFile: false,
			fullPath: "/subfolder",
			childCount: 3,
			format: 0,
			version: 0n,
			revision: 0n,
			locked: false,
			namespaceLocked: false,
		}),
	];

	it("renders files and folders correctly", () => {
		render(
			<MemoryRouter>
				<DirectoryTable
					namespace="default"
					currentPath="/"
					entries={mockEntries}
					isLoading={false}
					sorting={[]}
					onSortingChange={() => {}}
				/>
			</MemoryRouter>,
		);

		expect(screen.getByText("my-config")).toBeInTheDocument();
		expect(screen.getByText("subfolder")).toBeInTheDocument();
		expect(screen.getByText("JSON")).toBeInTheDocument();
		expect(screen.getByText("3 items")).toBeInTheDocument();
	});

	it("shows lock icon when entry is locked", () => {
		const lockedEntries = [
			create(DirectoryEntrySchema, { ...mockEntries[0], locked: true }),
		];
		render(
			<MemoryRouter>
				<DirectoryTable
					namespace="default"
					currentPath="/"
					entries={lockedEntries}
					isLoading={false}
					sorting={[]}
					onSortingChange={() => {}}
				/>
			</MemoryRouter>,
		);

		expect(screen.getByLabelText("Config is locked")).toBeInTheDocument();
	});

	it("navigates to browse for folder and config for file", async () => {
		const navigate = vi.fn();
		vi.mocked(useNavigate).mockReturnValue(navigate);
		const user = userEvent.setup();

		render(
			<MemoryRouter>
				<DirectoryTable
					namespace="default"
					currentPath="/"
					entries={mockEntries}
					isLoading={false}
					sorting={[]}
					onSortingChange={() => {}}
				/>
			</MemoryRouter>,
		);

		await user.click(screen.getByText("subfolder"));
		expect(navigate).toHaveBeenCalledWith("/browse/default/subfolder");

		await user.click(screen.getByText("my-config"));
		expect(navigate).toHaveBeenCalledWith("/config/default/my-config");
	});

	it("renders empty state with action button", async () => {
		const navigate = vi.fn();
		vi.mocked(useNavigate).mockReturnValue(navigate);
		const user = userEvent.setup();

		render(
			<MemoryRouter>
				<DirectoryTable
					namespace="default"
					currentPath="/"
					entries={[]}
					isLoading={false}
					sorting={[]}
					onSortingChange={() => {}}
				/>
			</MemoryRouter>,
		);

		expect(screen.getByText("Empty directory")).toBeInTheDocument();
		const newBtn = screen.getByRole("button", { name: "New Config" });
		await user.click(newBtn);
		expect(navigate).toHaveBeenCalledWith("/config/new/default");
	});
});

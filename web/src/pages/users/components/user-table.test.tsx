import { create } from "@bufbuild/protobuf";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, test, vi } from "vitest";
import { UserSchema } from "@/gen/elara/user/v1/user_pb";
import { TestProviders } from "@/test/test-utils";
import { UserTable } from "./user-table";

const mockUser1 = create(UserSchema, {
	email: "alice@example.com",
	displayName: "Alice",
	identities: [],
});

const mockUser2 = create(UserSchema, {
	email: "bob@example.com",
	displayName: "Bob",
	identities: [{ provider: "oidc", subject: "bob-sub" }],
});

describe("UserTable", () => {
	test("renders user rows", () => {
		render(
			<TestProviders>
				<UserTable
					users={[mockUser1, mockUser2]}
					isLoading={false}
					onRowClick={vi.fn()}
				/>
			</TestProviders>,
		);

		expect(screen.getByText("alice@example.com")).toBeInTheDocument();
		expect(screen.getByText("Alice")).toBeInTheDocument();
		expect(screen.getByText("bob@example.com")).toBeInTheDocument();
		expect(screen.getByText("Bob")).toBeInTheDocument();
	});

	test("shows empty state when no users and no query", () => {
		render(
			<TestProviders>
				<UserTable users={[]} isLoading={false} onRowClick={vi.fn()} />
			</TestProviders>,
		);

		expect(screen.getByText("No users")).toBeInTheDocument();
	});

	test("shows no results message when query provided", () => {
		render(
			<TestProviders>
				<UserTable
					users={[]}
					isLoading={false}
					query="alice"
					onRowClick={vi.fn()}
				/>
			</TestProviders>,
		);

		expect(screen.getByText("No users found")).toBeInTheDocument();
		expect(screen.getByText(/No results for "alice"/)).toBeInTheDocument();
	});

	test("calls onRowClick when row is clicked", async () => {
		const user = userEvent.setup();
		const handleClick = vi.fn();

		render(
			<TestProviders>
				<UserTable
					users={[mockUser1]}
					isLoading={false}
					onRowClick={handleClick}
				/>
			</TestProviders>,
		);

		await user.click(screen.getByText("alice@example.com"));
		expect(handleClick).toHaveBeenCalledWith(
			expect.objectContaining({ email: "alice@example.com" }),
			expect.anything(),
		);
	});
});

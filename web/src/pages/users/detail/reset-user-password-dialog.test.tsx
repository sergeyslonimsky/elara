import { create } from "@bufbuild/protobuf";
import { useMutation } from "@connectrpc/connect-query";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, test, vi } from "vitest";
import { UserSchema } from "@/gen/elara/user/v1/user_pb";
import { TestProviders } from "@/test/test-utils";
import { ResetUserPasswordDialog } from "./reset-user-password-dialog";

vi.mock("@connectrpc/connect-query", async (importOriginal) => {
	const actual = await importOriginal<Record<string, unknown>>();
	return {
		...actual,
		useMutation: vi.fn(() => ({
			mutate: vi.fn(),
			mutateAsync: vi.fn(),
			isPending: false,
		})),
	};
});

vi.mock("sonner", async (importOriginal) => {
	const actual = await importOriginal<Record<string, unknown>>();
	return {
		...actual,
		toast: { success: vi.fn(), error: vi.fn() },
	};
});

const mockUser = create(UserSchema, {
	email: "alice@example.com",
	name: "Alice",
});

describe("ResetUserPasswordDialog", () => {
	beforeEach(() => {
		vi.clearAllMocks();
	});

	test("renders when open", () => {
		render(
			<TestProviders>
				<ResetUserPasswordDialog
					user={mockUser}
					open={true}
					onOpenChange={vi.fn()}
				/>
			</TestProviders>,
		);

		expect(
			screen.getByRole("heading", { name: /reset password/i }),
		).toBeInTheDocument();
		expect(screen.getByText(/alice@example.com/)).toBeInTheDocument();
	});

	test("submit button disabled until valid password entered", async () => {
		const ue = userEvent.setup();

		render(
			<TestProviders>
				<ResetUserPasswordDialog
					user={mockUser}
					open={true}
					onOpenChange={vi.fn()}
				/>
			</TestProviders>,
		);

		const button = screen.getByRole("button", { name: /reset password/i });
		expect(button).toBeDisabled();

		await ue.type(screen.getByLabelText(/new password/i), "short");
		expect(button).toBeDisabled();

		await ue.clear(screen.getByLabelText(/new password/i));
		await ue.type(screen.getByLabelText(/new password/i), "longpassword123");
		expect(button).not.toBeDisabled();
	});

	test("shows inline error for short password", async () => {
		const ue = userEvent.setup();

		render(
			<TestProviders>
				<ResetUserPasswordDialog
					user={mockUser}
					open={true}
					onOpenChange={vi.fn()}
				/>
			</TestProviders>,
		);

		await ue.type(screen.getByLabelText(/new password/i), "short");

		expect(screen.getByText(/at least 8 characters/i)).toBeInTheDocument();
	});

	test("calls mutation with correct args", async () => {
		const ue = userEvent.setup();
		const mockMutate = vi.fn();

		vi.mocked(useMutation).mockReturnValue({
			mutate: mockMutate,
			mutateAsync: vi.fn(),
			isPending: false,
		} as unknown as ReturnType<typeof useMutation>);

		render(
			<TestProviders>
				<ResetUserPasswordDialog
					user={mockUser}
					open={true}
					onOpenChange={vi.fn()}
				/>
			</TestProviders>,
		);

		await ue.type(screen.getByLabelText(/new password/i), "newpassword123");
		await ue.click(screen.getByRole("button", { name: /reset password/i }));

		expect(mockMutate).toHaveBeenCalledWith(
			expect.objectContaining({
				email: "alice@example.com",
				newPassword: "newpassword123",
			}),
		);
	});
});

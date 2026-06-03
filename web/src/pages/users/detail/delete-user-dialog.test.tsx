import { create } from "@bufbuild/protobuf";
import { useMutation } from "@connectrpc/connect-query";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, test, vi } from "vitest";
import { UserSchema } from "@/gen/elara/user/v1/user_pb";
import { TestProviders } from "@/test/test-utils";
import { DeleteUserDialog } from "./delete-user-dialog";

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

const navigateSpy = vi.fn();
vi.mock("react-router", async (importOriginal) => {
	const actual = await importOriginal<Record<string, unknown>>();
	return {
		...actual,
		useNavigate: () => navigateSpy,
	};
});

vi.mock("@tanstack/react-query", async (importOriginal) => {
	const actual = await importOriginal<Record<string, unknown>>();
	return {
		...actual,
		useQueryClient: vi.fn(() => ({ invalidateQueries: vi.fn() })),
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
	id: "00000000-0000-0000-0000-00000000000a",
	email: "alice@example.com",
	displayName: "Alice",
});

describe("DeleteUserDialog", () => {
	beforeEach(() => {
		vi.clearAllMocks();
	});

	test("delete button disabled until correct email typed", async () => {
		const ue = userEvent.setup();

		render(
			<TestProviders>
				<DeleteUserDialog user={mockUser} open={true} onOpenChange={vi.fn()} />
			</TestProviders>,
		);

		const button = screen.getByRole("button", { name: /delete user/i });
		expect(button).toBeDisabled();

		await ue.type(screen.getByLabelText(/confirm email/i), "wrong@example.com");
		expect(button).toBeDisabled();

		await ue.clear(screen.getByLabelText(/confirm email/i));
		await ue.type(screen.getByLabelText(/confirm email/i), "alice@example.com");
		expect(button).not.toBeDisabled();
	});

	test("calls mutation with correct args on submit", async () => {
		const ue = userEvent.setup();
		const mockMutate = vi.fn();

		vi.mocked(useMutation).mockReturnValue({
			mutate: mockMutate,
			mutateAsync: vi.fn(),
			isPending: false,
		} as unknown as ReturnType<typeof useMutation>);

		render(
			<TestProviders>
				<DeleteUserDialog user={mockUser} open={true} onOpenChange={vi.fn()} />
			</TestProviders>,
		);

		await ue.type(screen.getByLabelText(/confirm email/i), "alice@example.com");
		await ue.click(screen.getByRole("button", { name: /delete user/i }));

		expect(mockMutate).toHaveBeenCalledWith(
			expect.objectContaining({
				userId: "00000000-0000-0000-0000-00000000000a",
			}),
		);
	});

	test("confirm field tolerates surrounding whitespace", async () => {
		const ue = userEvent.setup();

		render(
			<TestProviders>
				<DeleteUserDialog user={mockUser} open={true} onOpenChange={vi.fn()} />
			</TestProviders>,
		);

		await ue.type(
			screen.getByLabelText(/confirm email/i),
			"  alice@example.com  ",
		);

		expect(
			screen.getByRole("button", { name: /delete user/i }),
		).not.toBeDisabled();
	});

	test("navigates to /users after successful delete", async () => {
		const ue = userEvent.setup();
		navigateSpy.mockClear();

		let onSuccess: (() => void) | undefined;
		vi.mocked(useMutation).mockImplementation((_method, opts) => {
			onSuccess = opts?.onSuccess as () => void;
			return {
				mutate: vi.fn(() => onSuccess?.()),
				mutateAsync: vi.fn(),
				isPending: false,
			} as unknown as ReturnType<typeof useMutation>;
		});

		render(
			<TestProviders>
				<DeleteUserDialog user={mockUser} open={true} onOpenChange={vi.fn()} />
			</TestProviders>,
		);

		await ue.type(screen.getByLabelText(/confirm email/i), "alice@example.com");
		await ue.click(screen.getByRole("button", { name: /delete user/i }));

		expect(navigateSpy).toHaveBeenCalledWith("/users");
	});
});

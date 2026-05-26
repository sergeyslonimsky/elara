import { create } from "@bufbuild/protobuf";
import { useMutation } from "@connectrpc/connect-query";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, test, vi } from "vitest";
import { useAuth } from "@/components/auth-provider";
import { AuthType } from "@/gen/elara/auth/v1/auth_pb";
import { UserSchema } from "@/gen/elara/user/v1/user_pb";
import { TestProviders } from "@/test/test-utils";
import { UserDetailHeader } from "./user-detail-header";

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

vi.mock("@/components/auth-provider", async (importOriginal) => {
	const actual = await importOriginal<Record<string, unknown>>();
	return {
		...actual,
		useAuth: vi.fn(),
	};
});

vi.mock("react-router", async (importOriginal) => {
	const actual = await importOriginal<Record<string, unknown>>();
	return {
		...actual,
		useNavigate: () => vi.fn(),
	};
});

vi.mock("sonner", async (importOriginal) => {
	const actual = await importOriginal<Record<string, unknown>>();
	return {
		...actual,
		toast: { success: vi.fn(), error: vi.fn() },
	};
});

const regularUser = create(UserSchema, {
	email: "alice@example.com",
	name: "Alice",
	isSystem: false,
});

const systemUser = create(UserSchema, {
	email: "system@example.com",
	name: "System",
	isSystem: true,
});

describe("UserDetailHeader", () => {
	beforeEach(() => {
		vi.clearAllMocks();
		vi.mocked(useMutation).mockReturnValue({
			mutate: vi.fn(),
			mutateAsync: vi.fn(),
			isPending: false,
		} as unknown as ReturnType<typeof useMutation>);
	});

	test("hides actions menu when authType is not BASIC", () => {
		vi.mocked(useAuth).mockReturnValue({
			state: {
				status: "authenticated",
				ability: { can: () => true },
				authType: AuthType.NONE,
				user: { email: "admin@example.com", name: "Admin" },
			},
			logout: vi.fn(),
		} as unknown as ReturnType<typeof useAuth>);

		render(
			<TestProviders>
				<UserDetailHeader user={regularUser} />
			</TestProviders>,
		);

		expect(
			screen.queryByRole("button", { name: /user actions/i }),
		).not.toBeInTheDocument();
	});

	test("shows actions menu when authType is BASIC", () => {
		vi.mocked(useAuth).mockReturnValue({
			state: {
				status: "authenticated",
				ability: { can: () => true },
				authType: AuthType.BASIC,
				user: { email: "admin@example.com", name: "Admin" },
			},
			logout: vi.fn(),
		} as unknown as ReturnType<typeof useAuth>);

		render(
			<TestProviders>
				<UserDetailHeader user={regularUser} />
			</TestProviders>,
		);

		expect(
			screen.getByRole("button", { name: /user actions/i }),
		).toBeInTheDocument();
	});

	test("system user shows System badge", () => {
		vi.mocked(useAuth).mockReturnValue({
			state: {
				status: "authenticated",
				ability: { can: () => true },
				authType: AuthType.BASIC,
				user: { email: "admin@example.com", name: "Admin" },
			},
			logout: vi.fn(),
		} as unknown as ReturnType<typeof useAuth>);

		render(
			<TestProviders>
				<UserDetailHeader user={systemUser} />
			</TestProviders>,
		);

		expect(screen.getAllByText("System").length).toBeGreaterThan(0);
	});

	test("clicking Reset password opens dialog", async () => {
		const ue = userEvent.setup();
		vi.mocked(useAuth).mockReturnValue({
			state: {
				status: "authenticated",
				ability: { can: () => true },
				authType: AuthType.BASIC,
				user: { email: "admin@example.com", name: "Admin" },
			},
			logout: vi.fn(),
		} as unknown as ReturnType<typeof useAuth>);

		render(
			<TestProviders>
				<UserDetailHeader user={regularUser} />
			</TestProviders>,
		);

		await ue.click(screen.getByRole("button", { name: /user actions/i }));
		await ue.click(screen.getByRole("menuitem", { name: /reset password/i }));

		expect(
			screen.getByRole("heading", { name: /reset password/i }),
		).toBeInTheDocument();
	});
});

import { create } from "@bufbuild/protobuf";
import { useMutation } from "@connectrpc/connect-query";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, test, vi } from "vitest";
import { denyAllAbility } from "@/auth/ability";
import { AuthType } from "@/gen/elara/auth/v1/auth_pb";
import { UserSchema } from "@/gen/elara/user/v1/user_pb";
import { authenticatedContext, TestProviders } from "@/test/test-utils";
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
		const authContext = authenticatedContext(denyAllAbility, {
			authType: AuthType.NONE,
		});

		render(
			<TestProviders authContext={authContext}>
				<UserDetailHeader user={regularUser} />
			</TestProviders>,
		);

		expect(
			screen.queryByRole("button", { name: /user actions/i }),
		).not.toBeInTheDocument();
	});

	test("shows actions menu when authType is BASIC", () => {
		const authContext = authenticatedContext(denyAllAbility, {
			authType: AuthType.BASIC,
		});

		render(
			<TestProviders authContext={authContext}>
				<UserDetailHeader user={regularUser} />
			</TestProviders>,
		);

		expect(
			screen.getByRole("button", { name: /user actions/i }),
		).toBeInTheDocument();
	});

	test("system user shows System badge", () => {
		const authContext = authenticatedContext(denyAllAbility, {
			authType: AuthType.BASIC,
		});

		render(
			<TestProviders authContext={authContext}>
				<UserDetailHeader user={systemUser} />
			</TestProviders>,
		);

		expect(screen.getAllByText("System").length).toBeGreaterThan(0);
	});

	test("clicking Reset password opens dialog", async () => {
		const ue = userEvent.setup();
		const authContext = authenticatedContext(denyAllAbility, {
			authType: AuthType.BASIC,
		});

		render(
			<TestProviders authContext={authContext}>
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

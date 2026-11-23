import { create } from "@bufbuild/protobuf";
import { useMutation, useQuery } from "@connectrpc/connect-query";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, test, vi } from "vitest";
import { NamespaceSchema } from "@/gen/elara/namespace/v1/namespace_pb";
import { TokenSchema } from "@/gen/elara/token/v1/token_pb";
import {
	type CreateTokenRequest,
	CreateTokenResponseSchema,
} from "@/gen/elara/token/v1/token_service_pb";
import { TestProviders } from "@/test/test-utils";
import { CreateTokenDialog } from "./create-token-dialog";

vi.mock("@connectrpc/connect-query", async (importOriginal) => {
	const actual = await importOriginal<Record<string, unknown>>();
	return {
		...actual,
		useQuery: vi.fn(),
		useMutation: vi.fn(),
	};
});

const response = create(CreateTokenResponseSchema, {
	token: create(TokenSchema, {
		id: "tok-1",
		name: "billing-ci",
		username: "billing-ci-k7x2pq",
	}),
	rawToken: "elara_secret_value",
});

type MutationOptions = {
	onSuccess?: (
		data: typeof response,
		vars: Partial<CreateTokenRequest>,
	) => void;
};

const mutate = vi.fn();

function renderDialog() {
	render(
		<TestProviders>
			<CreateTokenDialog />
		</TestProviders>,
	);
}

describe("CreateTokenDialog", () => {
	beforeEach(() => {
		vi.clearAllMocks();
		vi.mocked(useQuery).mockReturnValue({
			data: {
				namespaces: [create(NamespaceSchema, { name: "production" })],
				pagination: { total: 1 },
			},
			isFetching: false,
		} as unknown as ReturnType<typeof useQuery>);
		vi.mocked(useMutation).mockImplementation(((
			_method: unknown,
			opts: MutationOptions,
		) => ({
			mutate: (vars: Partial<CreateTokenRequest>) => {
				mutate(vars);
				opts.onSuccess?.(response, vars);
			},
			isPending: false,
		})) as unknown as typeof useMutation);
	});

	test("labels the name field as a prefix with help text", async () => {
		const ue = userEvent.setup();
		renderDialog();

		await ue.click(screen.getByRole("button", { name: /new token/i }));

		const input = screen.getByLabelText("Name prefix");
		expect(input).toHaveAccessibleDescription(
			/server appends a random suffix/i,
		);
	});

	test.each([
		["Billing", /lowercase letters, digits/i],
		["ci:prod", /lowercase letters, digits/i],
		["-ci", /starting and ending with a letter or digit/i],
		["ci.", /starting and ending with a letter or digit/i],
		["a".repeat(49), /at most 48 characters/i],
	])("rejects invalid prefix %s", async (value, message) => {
		const ue = userEvent.setup();
		renderDialog();

		await ue.click(screen.getByRole("button", { name: /new token/i }));
		await ue.type(screen.getByLabelText("Name prefix"), value);

		expect(await screen.findByRole("alert")).toHaveTextContent(message);
		expect(screen.getByRole("button", { name: "Create" })).toBeDisabled();
	});

	test("shows a username preview for a valid prefix", async () => {
		const ue = userEvent.setup();
		renderDialog();

		await ue.click(screen.getByRole("button", { name: /new token/i }));
		await ue.type(screen.getByLabelText("Name prefix"), "billing-ci");

		expect(screen.getByText("billing-ci-xxxxxx")).toBeInTheDocument();
		expect(screen.queryByRole("alert")).not.toBeInTheDocument();
	});

	test("reveals username, token and etcdctl example after creation", async () => {
		const ue = userEvent.setup();
		renderDialog();

		await ue.click(screen.getByRole("button", { name: /new token/i }));
		await ue.type(screen.getByLabelText("Name prefix"), "billing-ci");
		await ue.click(screen.getByRole("combobox", { name: /namespaces/i }));
		await ue.click(screen.getByRole("button", { name: "production" }));
		await ue.keyboard("{Escape}");
		await ue.click(screen.getByRole("button", { name: "Create" }));

		expect(mutate).toHaveBeenCalledWith(
			expect.objectContaining({
				name: "billing-ci",
				namespaces: ["production"],
			}),
		);

		const dialog = await screen.findByRole("dialog");
		expect(within(dialog).getByText("Token created")).toBeInTheDocument();
		expect(within(dialog).getByLabelText("Username")).toHaveValue(
			"billing-ci-k7x2pq",
		);
		expect(within(dialog).getByLabelText("Token")).toHaveValue(
			"elara_secret_value",
		);
		expect(within(dialog).getByLabelText("Example")).toHaveValue(
			"etcdctl --endpoints=<host>:2379 --user billing-ci-k7x2pq:elara_secret_value get <key>",
		);
		expect(within(dialog).getByRole("alert")).toHaveTextContent(
			/shown only once/i,
		);
		expect(
			within(dialog).getByRole("button", { name: "Copy username" }),
		).toBeInTheDocument();
		expect(
			within(dialog).getByRole("button", { name: "Copy token" }),
		).toBeInTheDocument();
	});
});

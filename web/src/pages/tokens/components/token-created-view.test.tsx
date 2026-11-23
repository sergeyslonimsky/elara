import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, test, vi } from "vitest";
import { Dialog, DialogContent } from "@/components/ui/dialog";
import { TestProviders } from "@/test/test-utils";
import { buildEtcdctlExample, TokenCreatedView } from "./token-created-view";

function renderView(onDone = vi.fn()) {
	render(
		<TestProviders>
			<Dialog open>
				<DialogContent>
					<TokenCreatedView
						username="ci-k7x2pq"
						rawToken="raw-secret"
						onDone={onDone}
					/>
				</DialogContent>
			</Dialog>
		</TestProviders>,
	);
}

describe("TokenCreatedView", () => {
	test("builds the etcdctl example", () => {
		expect(buildEtcdctlExample("ci-k7x2pq", "raw-secret")).toBe(
			"etcdctl --endpoints=<host>:2379 --user ci-k7x2pq:raw-secret get <key>",
		);
	});

	test("copies the username and token to the clipboard", async () => {
		const ue = userEvent.setup();
		renderView();

		await ue.click(screen.getByRole("button", { name: "Copy username" }));
		expect(await navigator.clipboard.readText()).toBe("ci-k7x2pq");

		await ue.click(screen.getByRole("button", { name: "Copy token" }));
		expect(await navigator.clipboard.readText()).toBe("raw-secret");

		await ue.click(screen.getByRole("button", { name: "Copy example" }));
		expect(await navigator.clipboard.readText()).toBe(
			buildEtcdctlExample("ci-k7x2pq", "raw-secret"),
		);
	});

	test("calls onDone when Done is clicked", async () => {
		const ue = userEvent.setup();
		const onDone = vi.fn();
		renderView(onDone);

		await ue.click(screen.getByRole("button", { name: "Done" }));
		expect(onDone).toHaveBeenCalledOnce();
	});
});

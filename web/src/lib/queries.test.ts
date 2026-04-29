import { describe, expect, it, vi } from "vitest";
import { invalidate, invalidateAllConfigData } from "./queries";

vi.mock("@connectrpc/connect-query", () => ({
	createConnectQueryKey: () => ["mock-key"],
}));

vi.mock(
	"@/gen/elara/config/v1/config_service-ConfigService_connectquery",
	() => ({
		getConfig: {},
		getConfigHistory: {},
		listConfigs: {},
	}),
);

vi.mock(
	"@/gen/elara/config/v1/schema_service-SchemaService_connectquery",
	() => ({
		getSchema: {},
		listSchemas: {},
	}),
);

vi.mock(
	"@/gen/elara/namespace/v1/namespace_service-NamespaceService_connectquery",
	() => ({
		listNamespaces: {},
	}),
);

vi.mock(
	"@/gen/elara/webhook/v1/webhook_service-WebhookService_connectquery",
	() => ({
		listWebhooks: {},
	}),
);

describe("queries", () => {
	describe("invalidate", () => {
		it("calls client.invalidateQueries once with the correct key", () => {
			const mockClient = { invalidateQueries: vi.fn() };

			invalidate(mockClient as never, "webhooks");

			expect(mockClient.invalidateQueries).toHaveBeenCalledTimes(1);
			expect(mockClient.invalidateQueries).toHaveBeenCalledWith({
				queryKey: ["mock-key"],
			});
		});
	});

	describe("invalidateAllConfigData", () => {
		it("calls client.invalidateQueries three times for configs, config, and configHistory", () => {
			const mockClient = { invalidateQueries: vi.fn() };

			invalidateAllConfigData(mockClient as never);

			expect(mockClient.invalidateQueries).toHaveBeenCalledTimes(3);
		});
	});
});

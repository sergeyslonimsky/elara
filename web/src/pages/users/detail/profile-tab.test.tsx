import { create } from "@bufbuild/protobuf";
import { render, screen } from "@testing-library/react";
import { describe, expect, test } from "vitest";
import { UserSchema } from "@/gen/elara/user/v1/user_pb";
import { TestProviders } from "@/test/test-utils";
import { ProfileTab } from "./profile-tab";

describe("ProfileTab", () => {
	test("renders user fields", () => {
		const user = create(UserSchema, {
			email: "alice@example.com",
			name: "Alice",
			provider: "oidc",
		});

		render(
			<TestProviders>
				<ProfileTab user={user} />
			</TestProviders>,
		);

		expect(screen.getByText("alice@example.com")).toBeInTheDocument();
		expect(screen.getByText("Alice")).toBeInTheDocument();
		expect(screen.getByText("oidc")).toBeInTheDocument();
	});

	test("provider falls back to 'internal' when empty", () => {
		const user = create(UserSchema, {
			email: "alice@example.com",
			name: "Alice",
			provider: "",
		});

		render(
			<TestProviders>
				<ProfileTab user={user} />
			</TestProviders>,
		);

		expect(screen.getByText("internal")).toBeInTheDocument();
	});

	test("last login shows 'Never' when timestamp is undefined", () => {
		const user = create(UserSchema, {
			email: "alice@example.com",
			name: "Alice",
		});

		render(
			<TestProviders>
				<ProfileTab user={user} />
			</TestProviders>,
		);

		expect(screen.getByText("Never")).toBeInTheDocument();
	});
});

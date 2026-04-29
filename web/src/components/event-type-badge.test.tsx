import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { EventType } from "@/gen/elara/config/v1/config_pb";
import { EventTypeBadge } from "./event-type-badge";

describe("EventTypeBadge", () => {
	it("renders CREATED variant", () => {
		render(<EventTypeBadge type={EventType.CREATED} />);
		expect(screen.getByText("Created")).toBeInTheDocument();
	});

	it("renders UPDATED variant", () => {
		render(<EventTypeBadge type={EventType.UPDATED} />);
		expect(screen.getByText("Updated")).toBeInTheDocument();
	});

	it("renders DELETED variant", () => {
		render(<EventTypeBadge type={EventType.DELETED} />);
		expect(screen.getByText("Deleted")).toBeInTheDocument();
	});

	it("renders LOCKED variant", () => {
		render(<EventTypeBadge type={EventType.LOCKED} />);
		expect(screen.getByText("Locked")).toBeInTheDocument();
	});

	it("renders UNKNOWN for unexpected types", () => {
		render(<EventTypeBadge type={999 as EventType} />);
		expect(screen.getByText("Unknown")).toBeInTheDocument();
	});
});

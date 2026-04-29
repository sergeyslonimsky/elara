import { render } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { SkeletonList } from "./skeleton-list";

describe("SkeletonList", () => {
	it("renders the correct number of skeletons", () => {
		const { container } = render(<SkeletonList count={5} />);
		// Each skeleton has data-slot="skeleton"
		const skeletons = container.querySelectorAll('[data-slot="skeleton"]');
		expect(skeletons.length).toBe(5);
	});

	it("applies custom wrapperClassName", () => {
		const { container } = render(
			<SkeletonList count={1} wrapperClassName="custom-wrapper" />,
		);
		expect(container.firstChild).toHaveClass("custom-wrapper");
	});
});

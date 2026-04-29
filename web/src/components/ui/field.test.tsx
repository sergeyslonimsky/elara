import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import {
	Field,
	FieldContent,
	FieldDescription,
	FieldError,
	FieldGroup,
	FieldLabel,
	FieldLegend,
	FieldSeparator,
	FieldSet,
	FieldTitle,
} from "./field";

describe("FieldSet", () => {
	it("renders children", () => {
		render(
			<FieldSet data-testid="fieldset">
				<span>Child content</span>
			</FieldSet>,
		);
		expect(screen.getByTestId("fieldset")).toBeInTheDocument();
		expect(screen.getByText("Child content")).toBeInTheDocument();
	});

	it("applies custom className", () => {
		render(<FieldSet className="custom-class" data-testid="fieldset" />);
		expect(screen.getByTestId("fieldset")).toHaveClass("custom-class");
	});
});

describe("FieldLegend", () => {
	it("renders legend content", () => {
		render(<FieldLegend>My Legend</FieldLegend>);
		expect(screen.getByText("My Legend")).toBeInTheDocument();
	});

	it("applies default legend variant", () => {
		render(<FieldLegend data-testid="legend">Legend</FieldLegend>);
		const legend = screen.getByTestId("legend");
		expect(legend).toHaveAttribute("data-variant", "legend");
		// Assert on the full conditional class string
		expect(legend).toHaveClass("data-[variant=legend]:text-base");
	});

	it("applies label variant", () => {
		render(
			<FieldLegend variant="label" data-testid="legend">
				Label
			</FieldLegend>,
		);
		const legend = screen.getByTestId("legend");
		expect(legend).toHaveAttribute("data-variant", "label");
		// Assert on the full conditional class string
		expect(legend).toHaveClass("data-[variant=label]:text-sm");
	});

	it("applies custom className", () => {
		render(<FieldLegend className="custom-class">Legend</FieldLegend>);
		expect(screen.getByText("Legend")).toHaveClass("custom-class");
	});
});

describe("FieldGroup", () => {
	it("renders children", () => {
		render(
			<FieldGroup data-testid="fieldgroup">
				<span>Grouped content</span>
			</FieldGroup>,
		);
		expect(screen.getByTestId("fieldgroup")).toBeInTheDocument();
		expect(screen.getByText("Grouped content")).toBeInTheDocument();
	});

	it("applies custom className", () => {
		render(<FieldGroup className="custom-class" data-testid="fieldgroup" />);
		expect(screen.getByTestId("fieldgroup")).toHaveClass("custom-class");
	});
});

describe("Field", () => {
	it("renders children", () => {
		render(
			<Field data-testid="field">
				<span>Field content</span>
			</Field>,
		);
		expect(screen.getByTestId("field")).toBeInTheDocument();
		expect(screen.getByText("Field content")).toBeInTheDocument();
	});

	it("applies default vertical orientation", () => {
		render(<Field data-testid="field" />);
		const field = screen.getByTestId("field");
		expect(field).toHaveAttribute("data-orientation", "vertical");
		expect(field).toHaveClass("flex-col");
	});

	it("applies horizontal orientation", () => {
		render(<Field orientation="horizontal" data-testid="field" />);
		const field = screen.getByTestId("field");
		expect(field).toHaveAttribute("data-orientation", "horizontal");
		expect(field).toHaveClass("flex-row");
	});

	it("applies responsive orientation", () => {
		render(<Field orientation="responsive" data-testid="field" />);
		const field = screen.getByTestId("field");
		expect(field).toHaveAttribute("data-orientation", "responsive");
		expect(field).toHaveClass("flex-col"); // Default for small screens
	});

	it("applies custom className", () => {
		render(<Field className="custom-class" data-testid="field" />);
		expect(screen.getByTestId("field")).toHaveClass("custom-class");
	});
});

describe("FieldContent", () => {
	it("renders children", () => {
		render(
			<FieldContent data-testid="fieldcontent">
				<span>Content area</span>
			</FieldContent>,
		);
		expect(screen.getByTestId("fieldcontent")).toBeInTheDocument();
		expect(screen.getByText("Content area")).toBeInTheDocument();
	});

	it("applies custom className", () => {
		render(<FieldContent className="custom-class" data-testid="fieldcontent" />);
		expect(screen.getByTestId("fieldcontent")).toHaveClass("custom-class");
	});
});

describe("FieldLabel", () => {
	it("renders label content", () => {
		render(<FieldLabel>My Label</FieldLabel>);
		expect(screen.getByText("My Label")).toBeInTheDocument();
	});

	it("applies custom className", () => {
		render(<FieldLabel className="custom-class">Label</FieldLabel>);
		expect(screen.getByText("Label")).toHaveClass("custom-class");
	});
});

describe("FieldTitle", () => {
	it("renders title content", () => {
		render(<FieldTitle>My Title</FieldTitle>);
		expect(screen.getByText("My Title")).toBeInTheDocument();
	});

	it("applies custom className", () => {
		render(<FieldTitle className="custom-class">Title</FieldTitle>);
		expect(screen.getByText("Title")).toHaveClass("custom-class");
	});
});

describe("FieldDescription", () => {
	it("renders description content", () => {
		render(<FieldDescription>Description text</FieldDescription>);
		expect(screen.getByText("Description text")).toBeInTheDocument();
	});

	it("applies custom className", () => {
		render(<FieldDescription className="custom-class">Description</FieldDescription>);
		expect(screen.getByText("Description")).toHaveClass("custom-class");
	});
});

describe("FieldSeparator", () => {
	it("renders without children", () => {
		render(<FieldSeparator data-testid="separator" />);
		const separator = screen.getByTestId("separator");
		expect(separator).toBeInTheDocument();
		expect(separator).toHaveAttribute("data-content", "false");
		expect(screen.queryByTestId("field-separator-content")).not.toBeInTheDocument();
	});

	it("renders with children", () => {
		render(<FieldSeparator>OR</FieldSeparator>);
		const separatorContent = screen.getByText("OR");
		expect(separatorContent).toBeInTheDocument();
		expect(separatorContent.parentElement).toHaveAttribute("data-content", "true");
	});

	it("applies custom className", () => {
		render(<FieldSeparator className="custom-class" data-testid="separator" />);
		expect(screen.getByTestId("separator")).toHaveClass("custom-class");
	});
});

describe("FieldError", () => {
	it("renders children when provided", () => {
		render(<FieldError>Custom Error Message</FieldError>);
		expect(screen.getByText("Custom Error Message")).toBeInTheDocument();
	});

	it("renders single error message from errors prop", () => {
		render(<FieldError errors={[{ message: "Error 1" }]} />);
		expect(screen.getByText("Error 1")).toBeInTheDocument();
	});

	it("renders multiple error messages as a list from errors prop", () => {
		render(
			<FieldError
				errors={[{ message: "Error A" }, { message: "Error B" }]}
			/>,
		);
		expect(screen.getByRole("list")).toBeInTheDocument();
		expect(screen.getByText("Error A")).toBeInTheDocument();
		expect(screen.getByText("Error B")).toBeInTheDocument();
	});

	it("does not render if no children or errors are provided", () => {
		const { container } = render(<FieldError />);
		expect(container).toBeEmptyDOMElement();
	});

	it("applies custom className", () => {
		render(<FieldError className="custom-class">Error</FieldError>);
		expect(screen.getByText("Error")).toHaveClass("custom-class");
	});
});

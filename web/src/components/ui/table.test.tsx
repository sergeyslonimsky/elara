import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import {
	Table,
	TableBody,
	TableCaption,
	TableCell,
	TableFooter,
	TableHead,
	TableHeader,
	TableRow,
} from "./table";

describe("Table", () => {
	it("renders correctly with all parts", () => {
		render(
			<Table>
				<TableCaption>Test Caption</TableCaption>
				<TableHeader>
					<TableRow>
						<TableHead>Header 1</TableHead>
					</TableRow>
				</TableHeader>
				<TableBody>
					<TableRow>
						<TableCell>Cell 1</TableCell>
					</TableRow>
				</TableBody>
				<TableFooter>
					<TableRow>
						<TableCell>Footer 1</TableCell>
					</TableRow>
				</TableFooter>
			</Table>,
		);

		expect(screen.getByRole("table")).toBeInTheDocument();
		expect(screen.getByText("Test Caption")).toBeInTheDocument();
		expect(screen.getByText("Header 1")).toBeInTheDocument();
		expect(screen.getByText("Cell 1")).toBeInTheDocument();
		expect(screen.getByText("Footer 1")).toBeInTheDocument();
	});

	it("applies custom classNames", () => {
		render(
			<Table className="table-class">
				<TableHeader className="header-class">
					<TableRow className="row-class">
						<TableHead className="head-class">H</TableHead>
					</TableRow>
				</TableHeader>
				<TableBody className="body-class">
					<TableRow>
						<TableCell className="cell-class">C</TableCell>
					</TableRow>
				</TableBody>
			</Table>,
		);

		expect(screen.getByRole("table")).toHaveClass("table-class");
		expect(screen.getByText("H").closest('[data-slot="table-header"]')).toHaveClass("header-class");
		expect(screen.getByText("C").closest('[data-slot="table-body"]')).toHaveClass("body-class");
	});
});

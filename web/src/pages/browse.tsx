import { timestampDate } from "@bufbuild/protobuf/wkt";
import { useQuery } from "@connectrpc/connect-query";
import {
	type ColumnDef,
	flexRender,
	getCoreRowModel,
	type SortingState,
	useReactTable,
} from "@tanstack/react-table";
import { Database, FilePlus, Folder, Search, X } from "lucide-react";
import { useEffect, useState } from "react";
import { Link, useNavigate, useParams } from "react-router";
import { AppHeader } from "@/components/app-header";
import { DirectoryTable, sortingToParams } from "@/components/directory-table";
import { ErrorCard } from "@/components/error-card";
import { PaginationControls } from "@/components/pagination-controls";
import { PathBreadcrumb } from "@/components/path-breadcrumb";
import { SortableHeader } from "@/components/sortable-header";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import {
	Table,
	TableBody,
	TableCell,
	TableHead,
	TableHeader,
	TableRow,
} from "@/components/ui/table";
import { listConfigs } from "@/gen/elara/config/v1/config_service-ConfigService_connectquery";
import type { Namespace } from "@/gen/elara/namespace/v1/namespace_pb";
import { listNamespaces } from "@/gen/elara/namespace/v1/namespace_service-NamespaceService_connectquery";
import { timeAgo, tsToMs } from "@/lib/time";

const DEFAULT_PAGE_SIZE = 20;

const nsColumns: ColumnDef<Namespace>[] = [
	{
		accessorKey: "name",
		header: ({ column }) => (
			<SortableHeader column={column}>Name</SortableHeader>
		),
		cell: ({ row }) => (
			<div className="flex items-center gap-2 font-medium">
				<Folder className="h-4 w-4 shrink-0 text-blue-500" />
				{row.original.name}
			</div>
		),
	},
	{
		id: "type",
		header: "Type",
		cell: () => <Badge variant="outline">Namespace</Badge>,
		enableSorting: false,
	},
	{
		id: "info",
		header: "Info",
		cell: ({ row }) => (
			<span className="text-muted-foreground text-sm">
				{row.original.configCount} config
				{row.original.configCount !== 1 ? "s" : ""}
			</span>
		),
		enableSorting: false,
	},
	{
		id: "modified",
		accessorFn: (row) => tsToMs(row.updatedAt),
		header: ({ column }) => (
			<div className="text-right">
				<SortableHeader column={column}>Modified</SortableHeader>
			</div>
		),
		cell: ({ row }) => (
			<div className="text-right text-muted-foreground text-sm">
				{row.original.updatedAt &&
					timeAgo(timestampDate(row.original.updatedAt))}
			</div>
		),
	},
];

function SearchInput({
	value,
	onChange,
	onSearch,
	onClear,
	placeholder,
}: {
	value: string;
	onChange: (v: string) => void;
	onSearch: () => void;
	onClear: () => void;
	placeholder: string;
}) {
	return (
		<div className="relative w-72">
			<Search className="absolute top-2.5 left-2.5 h-4 w-4 text-muted-foreground" />
			<Input
				placeholder={placeholder}
				className="pl-8 pr-8"
				value={value}
				onChange={(e) => onChange(e.target.value)}
				onKeyDown={(e) => {
					if (e.key === "Enter") onSearch();
					if (e.key === "Escape" && value) onClear();
				}}
			/>
			{value && (
				<button
					type="button"
					className="absolute top-2.5 right-2.5 text-muted-foreground hover:text-foreground"
					onClick={onClear}
				>
					<X className="h-4 w-4" />
				</button>
			)}
		</div>
	);
}

function NamespaceList() {
	const [sorting, setSorting] = useState<SortingState>([]);
	const [pageSize, setPageSize] = useState(DEFAULT_PAGE_SIZE);
	const [offset, setOffset] = useState(0);
	const [searchInput, setSearchInput] = useState("");
	const [query, setQuery] = useState("");
	const navigate = useNavigate();

	const sortParams = sortingToParams(sorting);

	const { data, isLoading, error } = useQuery(listNamespaces, {
		pagination: { limit: pageSize, offset },
		sort: sortParams.field
			? { field: sortParams.field, direction: sortParams.direction }
			: undefined,
		query,
	});

	const total = data?.pagination?.total ?? 0;

	const handleSearch = () => {
		setQuery(searchInput);
		setOffset(0);
	};

	const handleClear = () => {
		setSearchInput("");
		setQuery("");
		setOffset(0);
	};

	const table = useReactTable({
		data: data?.namespaces ?? [],
		columns: nsColumns,
		state: { sorting },
		onSortingChange: (updater) => {
			const next = typeof updater === "function" ? updater(sorting) : updater;
			setSorting(next);
			setOffset(0);
		},
		getCoreRowModel: getCoreRowModel(),
		manualSorting: true,
	});

	return (
		<>
			<div className="flex items-center gap-3">
				<SearchInput
					value={searchInput}
					onChange={setSearchInput}
					onSearch={handleSearch}
					onClear={handleClear}
					placeholder="Search namespaces..."
				/>
				<Button variant="outline" size="sm" onClick={handleSearch}>
					Search
				</Button>
			</div>

			{error && <ErrorCard message={error.message} />}

			{isLoading ? (
				<div className="space-y-2 p-4">
					{Array.from({ length: 5 }).map((_, i) => (
						// biome-ignore lint/suspicious/noArrayIndexKey: skeleton
						<Skeleton key={i} className="h-10 w-full" />
					))}
				</div>
			) : total === 0 ? (
				<div className="flex flex-col items-center justify-center gap-3 py-16 text-muted-foreground">
					<Database className="h-12 w-12" />
					<p className="text-lg font-medium">
						{query ? "No namespaces found" : "No namespaces"}
					</p>
					<p className="text-sm">
						{query
							? `No results for "${query}"`
							: "Create a namespace to start managing configs"}
					</p>
					{!query && (
						<Button
							variant="outline"
							size="sm"
							render={<Link to="/namespaces" />}
						>
							Go to Namespaces
						</Button>
					)}
				</div>
			) : (
				<div className="rounded-xl border bg-card">
					<Table>
						<TableHeader>
							{table.getHeaderGroups().map((hg) => (
								<TableRow key={hg.id}>
									{hg.headers.map((header) => (
										<TableHead
											key={header.id}
											className={header.id === "name" ? "w-[50%]" : ""}
										>
											{header.isPlaceholder
												? null
												: flexRender(
														header.column.columnDef.header,
														header.getContext(),
													)}
										</TableHead>
									))}
								</TableRow>
							))}
						</TableHeader>
						<TableBody>
							{table.getRowModel().rows.map((row) => (
								<TableRow
									key={row.id}
									className="cursor-pointer"
									tabIndex={0}
									role="link"
									onClick={() => navigate(`/browse/${row.original.name}`)}
									onKeyDown={(e) => {
										if (e.key === "Enter" || e.key === " ") {
											e.preventDefault();
											navigate(`/browse/${row.original.name}`);
										}
									}}
								>
									{row.getVisibleCells().map((cell) => (
										<TableCell key={cell.id}>
											{flexRender(
												cell.column.columnDef.cell,
												cell.getContext(),
											)}
										</TableCell>
									))}
								</TableRow>
							))}
						</TableBody>
					</Table>
				</div>
			)}

			<PaginationControls
				total={total}
				pageSize={pageSize}
				offset={offset}
				onOffsetChange={setOffset}
				onPageSizeChange={(size) => {
					setPageSize(size);
					setOffset(0);
				}}
			/>
		</>
	);
}

export function BrowsePage() {
	const { namespace, "*": splat = "" } = useParams();
	const path = namespace ? `/${splat}` : undefined;
	const [offset, setOffset] = useState(0);
	const [pageSize, setPageSize] = useState(DEFAULT_PAGE_SIZE);
	const [sorting, setSorting] = useState<SortingState>([]);
	const [searchInput, setSearchInput] = useState("");
	const [query, setQuery] = useState("");

	// biome-ignore lint/correctness/useExhaustiveDependencies: reset on route change
	useEffect(() => {
		setOffset(0);
		setSorting([]);
		setSearchInput("");
		setQuery("");
	}, [namespace, path]);

	const sortParams = sortingToParams(sorting);

	const { data, isLoading, error } = useQuery(
		listConfigs,
		namespace
			? {
					namespace,
					path: path ?? "/",
					pagination: { limit: pageSize, offset },
					sort: sortParams.field
						? {
								field: sortParams.field,
								direction: sortParams.direction,
							}
						: undefined,
					query,
				}
			: undefined,
	);

	const total = data?.pagination?.total ?? 0;

	const handleSearch = () => {
		setQuery(searchInput);
		setOffset(0);
	};

	const handleClear = () => {
		setSearchInput("");
		setQuery("");
		setOffset(0);
	};

	if (!namespace) {
		return (
			<>
				<AppHeader />
				<div className="flex flex-1 flex-col gap-4 p-4 pt-0">
					<div className="mt-4">
						<PathBreadcrumb path="/" />
					</div>
					<NamespaceList />
				</div>
			</>
		);
	}

	const newConfigPath =
		path === "/"
			? `/config/new/${namespace}`
			: `/config/new/${namespace}${path}`;

	return (
		<>
			<AppHeader />
			<div className="flex flex-1 flex-col gap-4 p-4 pt-0">
				<div className="mt-4 flex items-center justify-between">
					<PathBreadcrumb namespace={namespace} path={path ?? "/"} />
					<Button size="sm" render={<Link to={newConfigPath} />}>
						<FilePlus className="mr-1 h-4 w-4" />
						New Config
					</Button>
				</div>

				<div className="flex items-center gap-3">
					<SearchInput
						value={searchInput}
						onChange={setSearchInput}
						onSearch={handleSearch}
						onClear={handleClear}
						placeholder="Search configs..."
					/>
					<Button variant="outline" size="sm" onClick={handleSearch}>
						Search
					</Button>
				</div>

				{error && <ErrorCard message={error.message} />}

				<div className="rounded-xl border bg-card">
					<DirectoryTable
						namespace={namespace}
						currentPath={path ?? "/"}
						entries={data?.entries ?? []}
						isLoading={isLoading}
						sorting={sorting}
						onSortingChange={(next) => {
							setSorting(next);
							setOffset(0);
						}}
					/>
				</div>

				<PaginationControls
					total={total}
					pageSize={pageSize}
					offset={offset}
					onOffsetChange={setOffset}
					onPageSizeChange={(size) => {
						setPageSize(size);
						setOffset(0);
					}}
				/>
			</div>
		</>
	);
}

import { useMutation } from "@connectrpc/connect-query";
import { useQueryClient } from "@tanstack/react-query";
import { Database, Pencil, Plus, Trash2 } from "lucide-react";
import { useState } from "react";
import { Link } from "react-router";
import { toast } from "sonner";
import { ErrorCard } from "@/components/error-card";
import { ExportDialog } from "@/components/export-dialog";
import { ImportDialog } from "@/components/import-dialog";
import { PageHeader } from "@/components/page-header";
import { SearchInput } from "@/components/search-input";
import {
	AlertDialog,
	AlertDialogAction,
	AlertDialogCancel,
	AlertDialogContent,
	AlertDialogDescription,
	AlertDialogFooter,
	AlertDialogHeader,
	AlertDialogTitle,
	AlertDialogTrigger,
} from "@/components/ui/alert-dialog";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
	Card,
	CardContent,
	CardDescription,
	CardHeader,
	CardTitle,
} from "@/components/ui/card";
import {
	Dialog,
	DialogContent,
	DialogFooter,
	DialogHeader,
	DialogTitle,
	DialogTrigger,
} from "@/components/ui/dialog";
import {
	Empty,
	EmptyDescription,
	EmptyHeader,
	EmptyMedia,
	EmptyTitle,
} from "@/components/ui/empty";
import { Field, FieldLabel } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import { Textarea } from "@/components/ui/textarea";
import {
	createNamespace,
	deleteNamespace,
	updateNamespace,
} from "@/gen/elara/namespace/v1/namespace_service-NamespaceService_connectquery";
import { useNamespaces } from "@/hooks/use-namespaces";
import { invalidateNamespaces } from "@/lib/queries";

function CreateDialog() {
	const [open, setOpen] = useState(false);
	const [name, setName] = useState("");
	const [description, setDescription] = useState("");
	const queryClient = useQueryClient();

	const mutation = useMutation(createNamespace, {
		onSuccess: () => {
			toast.success(`Namespace "${name}" created`);
			setOpen(false);
			setName("");
			setDescription("");
			void invalidateNamespaces(queryClient);
		},
		onError: (err) => toast.error(err.message),
	});

	return (
		<Dialog open={open} onOpenChange={setOpen}>
			<DialogTrigger render={<Button size="sm" />}>
				<Plus className="mr-1 h-4 w-4" />
				New Namespace
			</DialogTrigger>
			<DialogContent>
				<form
					onSubmit={(e) => {
						e.preventDefault();
						mutation.mutate({ name, description });
					}}
				>
					<DialogHeader>
						<DialogTitle>Create Namespace</DialogTitle>
					</DialogHeader>
					<div className="grid gap-4 py-4">
						<Field>
							<FieldLabel>Name</FieldLabel>
							<Input
								value={name}
								onChange={(e) => setName(e.target.value)}
								placeholder="production"
								required
							/>
						</Field>
						<Field>
							<FieldLabel>Description</FieldLabel>
							<Textarea
								value={description}
								onChange={(e) => setDescription(e.target.value)}
								placeholder="Production environment configs"
							/>
						</Field>
					</div>
					<DialogFooter>
						<Button type="submit" disabled={mutation.isPending || !name}>
							{mutation.isPending ? "Creating..." : "Create"}
						</Button>
					</DialogFooter>
				</form>
			</DialogContent>
		</Dialog>
	);
}

function EditDialog({
	name,
	currentDescription,
}: {
	name: string;
	currentDescription: string;
}) {
	const [open, setOpen] = useState(false);
	const [description, setDescription] = useState(currentDescription);
	const queryClient = useQueryClient();

	// Reset description when dialog opens with fresh props.
	const handleOpenChange = (isOpen: boolean) => {
		if (isOpen) setDescription(currentDescription);
		setOpen(isOpen);
	};

	const mutation = useMutation(updateNamespace, {
		onSuccess: () => {
			toast.success(`Namespace "${name}" updated`);
			setOpen(false);
			void invalidateNamespaces(queryClient);
		},
		onError: (err) => toast.error(err.message),
	});

	return (
		<Dialog open={open} onOpenChange={handleOpenChange}>
			<DialogTrigger render={<Button variant="ghost" size="icon-xs" />}>
				<Pencil className="h-3.5 w-3.5" />
			</DialogTrigger>
			<DialogContent>
				<form
					onSubmit={(e) => {
						e.preventDefault();
						mutation.mutate({ name, description });
					}}
				>
					<DialogHeader>
						<DialogTitle>Edit "{name}"</DialogTitle>
					</DialogHeader>
					<div className="grid gap-4 py-4">
						<Field>
							<FieldLabel>Description</FieldLabel>
							<Textarea
								value={description}
								onChange={(e) => setDescription(e.target.value)}
							/>
						</Field>
					</div>
					<DialogFooter>
						<Button type="submit" disabled={mutation.isPending}>
							{mutation.isPending ? "Saving..." : "Save"}
						</Button>
					</DialogFooter>
				</form>
			</DialogContent>
		</Dialog>
	);
}

function DeleteButton({ name }: { name: string }) {
	const queryClient = useQueryClient();

	const mutation = useMutation(deleteNamespace, {
		onSuccess: () => {
			toast.success(`Namespace "${name}" deleted`);
			void invalidateNamespaces(queryClient);
		},
		onError: (err) => toast.error(err.message),
	});

	return (
		<AlertDialog>
			<AlertDialogTrigger render={<Button variant="ghost" size="icon-xs" />}>
				<Trash2 className="h-3.5 w-3.5 text-destructive" />
			</AlertDialogTrigger>
			<AlertDialogContent>
				<AlertDialogHeader>
					<AlertDialogTitle>Delete namespace "{name}"?</AlertDialogTitle>
					<AlertDialogDescription>
						This action cannot be undone. The namespace must be empty (no
						configs).
					</AlertDialogDescription>
				</AlertDialogHeader>
				<AlertDialogFooter>
					<AlertDialogCancel>Cancel</AlertDialogCancel>
					<AlertDialogAction
						className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
						disabled={mutation.isPending}
						onClick={() => mutation.mutate({ name })}
					>
						{mutation.isPending ? "Deleting..." : "Delete"}
					</AlertDialogAction>
				</AlertDialogFooter>
			</AlertDialogContent>
		</AlertDialog>
	);
}

export function NamespacesPage() {
	const [searchInput, setSearchInput] = useState("");
	const [query, setQuery] = useState("");
	const { data, isLoading, error, refetch, isFetching } = useNamespaces(query);

	const handleSearch = () => {
		setQuery(searchInput);
	};

	const handleClear = () => {
		setSearchInput("");
		setQuery("");
	};

	return (
		<div className="flex flex-1 flex-col">
			<PageHeader
				title="Namespaces"
				onRefresh={() => void refetch()}
				isRefreshing={isFetching}
			>
				<SearchInput
					value={searchInput}
					onChange={setSearchInput}
					onSearch={handleSearch}
					onClear={handleClear}
					placeholder="Search namespaces..."
				/>
			</PageHeader>

			<div className="flex flex-1 flex-col gap-4 p-4">
				<div className="flex items-center justify-end gap-2">
					<ImportDialog />
					<ExportDialog />
					<CreateDialog />
				</div>

				{error && <ErrorCard message={error.message} />}

				<div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
					{isLoading &&
						Array.from({ length: 3 }).map((_, i) => (
							// biome-ignore lint/suspicious/noArrayIndexKey: skeleton placeholder
							<Skeleton key={i} className="h-32 rounded-xl" />
						))}

					{data?.namespaces.map((ns) => (
						<Card key={ns.name} className="rounded-xl">
							<CardHeader className="pb-2">
								<div className="flex items-start justify-between">
									<Link
										to={`/browse/${ns.name}`}
										className="flex items-center gap-2 hover:underline"
									>
										<Database className="h-4 w-4 text-muted-foreground" />
										<CardTitle className="text-base">{ns.name}</CardTitle>
									</Link>
									<div className="flex gap-1">
										<ExportDialog namespace={ns.name} />
										<ImportDialog namespace={ns.name} />
										<EditDialog
											name={ns.name}
											currentDescription={ns.description}
										/>
										{ns.name !== "default" && <DeleteButton name={ns.name} />}
									</div>
								</div>
								{ns.description && (
									<CardDescription>{ns.description}</CardDescription>
								)}
							</CardHeader>
							<CardContent>
								<Badge variant="secondary">
									{ns.configCount} config
									{ns.configCount !== 1 ? "s" : ""}
								</Badge>
							</CardContent>
						</Card>
					))}

					{data && data.namespaces.length === 0 && (
						<div className="col-span-full py-16">
							<Empty>
								<EmptyHeader>
									<EmptyMedia variant="icon">
										<Database />
									</EmptyMedia>
									<EmptyTitle>
										{query ? "No namespaces found" : "No namespaces"}
									</EmptyTitle>
									<EmptyDescription>
										{query
											? `No results for "${query}"`
											: "Create your first namespace to get started"}
									</EmptyDescription>
								</EmptyHeader>
							</Empty>
						</div>
					)}
				</div>
			</div>
		</div>
	);
}

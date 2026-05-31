import { create } from "@bufbuild/protobuf";
import { Code, ConnectError } from "@connectrpc/connect";
import { useMutation, useQuery } from "@connectrpc/connect-query";
import { useQueryClient } from "@tanstack/react-query";
import { useId, useMemo, useState } from "react";
import { toast } from "sonner";
import {
	canManageGroup,
	displayObject,
	formatAction,
	GROUP_DOMAIN_PREFIX,
	groupResource,
	NAMESPACE_DOMAIN_PREFIX,
	namespaceResource,
	WILDCARD_DOMAIN,
} from "@/auth/ability";
import { useAbility } from "@/auth/ability-context";
import { GroupFilter, NamespaceFilter } from "@/components/filters";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Label } from "@/components/ui/label";
import {
	Select,
	SelectContent,
	SelectItem,
	SelectTrigger,
	SelectValue,
} from "@/components/ui/select";
import {
	type PermissionAction,
	type PermissionAssignment,
	PermissionAssignmentSchema,
	type PermissionObject,
} from "@/gen/elara/common/v1/permission_pb";
import { ObjectScope } from "@/gen/elara/filter/v1/filter_service_pb";
import { getPermissionCatalog } from "@/gen/elara/filter/v1/filter_service-FilterService_connectquery";
import type { Group } from "@/gen/elara/group/v1/group_pb";
import { updateGroupPermissions } from "@/gen/elara/group/v1/group_service-GroupService_connectquery";
import { invalidate } from "@/lib/queries";
import { toastError } from "@/lib/toast";

interface PermissionsTabProps {
	group: Group;
	permissions: PermissionAssignment[];
}

function permKey(p: PermissionAssignment): string {
	return `${p.object}|${p.action}|${p.domain}`;
}

export function PermissionsTab({
	group,
	permissions,
}: Readonly<PermissionsTabProps>) {
	const ability = useAbility();
	const queryClient = useQueryClient();

	const canEdit = canManageGroup(ability, group);

	const { data: catalogData, isLoading: catalogLoading } = useQuery(
		getPermissionCatalog,
		{},
	);
	const catalog = catalogData?.entries ?? [];

	const [addPerms, setAddPerms] = useState<PermissionAssignment[]>([]);
	const [removeKeys, setRemoveKeys] = useState<Set<string>>(new Set());

	const hasChanges = addPerms.length > 0 || removeKeys.size > 0;

	const mutation = useMutation(updateGroupPermissions, {
		onSuccess: () => {
			toast.success("Permissions updated");
			setAddPerms([]);
			setRemoveKeys(new Set());
			invalidate(queryClient, "group");
			invalidate(queryClient, "groups");
		},
		onError: (err) => {
			const connectErr = ConnectError.from(err);
			if (connectErr.code === Code.FailedPrecondition) {
				toast.error(
					"Group changed since you loaded the page — refresh to retry.",
				);
			} else {
				toastError(err);
			}
		},
	});

	const handleSave = () => {
		mutation.mutate({
			groupId: group.id,
			add: addPerms,
			remove: permissions.filter((p) => removeKeys.has(permKey(p))),
			expectedPermissionsVersion: group.permissionsVersion,
		});
	};

	const stagePerm = (perm: PermissionAssignment) => {
		const key = permKey(perm);
		const exists =
			permissions.some((p) => permKey(p) === key) ||
			addPerms.some((p) => permKey(p) === key);
		if (exists) return;
		setAddPerms((prev) => [...prev, perm]);
	};

	return (
		<div className="mt-2 flex flex-col gap-3">
			<div className="rounded-xl border bg-card divide-y">
				{permissions.length === 0 && addPerms.length === 0 && (
					<p className="p-4 text-sm text-muted-foreground italic">
						No permissions assigned.
					</p>
				)}
				{permissions.map((p) => {
					const key = permKey(p);
					const isRemoved = removeKeys.has(key);
					return (
						<PermissionRow
							key={key}
							perm={p}
							muted={isRemoved}
							trailing={
								canEdit &&
								(isRemoved ? (
									<Button
										type="button"
										variant="ghost"
										size="sm"
										onClick={() =>
											setRemoveKeys((prev) => {
												const next = new Set(prev);
												next.delete(key);
												return next;
											})
										}
									>
										Undo
									</Button>
								) : (
									<Button
										type="button"
										variant="ghost"
										size="sm"
										className="text-destructive hover:text-destructive"
										onClick={() =>
											setRemoveKeys((prev) => new Set([...prev, key]))
										}
									>
										Remove
									</Button>
								))
							}
						/>
					);
				})}
				{addPerms.map((p) => {
					const key = permKey(p);
					return (
						<PermissionRow
							key={key}
							perm={p}
							additive
							trailing={
								<Button
									type="button"
									variant="ghost"
									size="sm"
									aria-label={`Un-stage permission ${displayObject(p.object)} ${p.domain}`}
									onClick={() =>
										setAddPerms((prev) =>
											prev.filter((a) => permKey(a) !== key),
										)
									}
								>
									×
								</Button>
							}
						/>
					);
				})}
			</div>

			{canEdit && (
				<AddPermissionForm
					catalog={catalog}
					catalogLoading={catalogLoading}
					onStage={stagePerm}
				/>
			)}

			{addPerms.length > 0 && (
				<div className="flex flex-wrap gap-1">
					{addPerms.map((p) => (
						<Badge
							key={permKey(p)}
							variant="outline"
							className="text-emerald-500"
						>
							+{displayObject(p.object)}:{displayDomain(p.domain)}
						</Badge>
					))}
				</div>
			)}

			<div className="flex justify-end">
				<Button
					size="sm"
					onClick={handleSave}
					disabled={!canEdit || !hasChanges || mutation.isPending}
				>
					{mutation.isPending ? "Saving..." : "Save changes"}
				</Button>
			</div>
		</div>
	);
}

// ---- PermissionRow ----------------------------------------------------------

interface PermissionRowProps {
	perm: PermissionAssignment;
	muted?: boolean;
	additive?: boolean;
	trailing?: React.ReactNode;
}

function PermissionRow({
	perm,
	muted,
	additive,
	trailing,
}: Readonly<PermissionRowProps>) {
	const wrapperClass = additive
		? "flex items-center justify-between p-3 text-sm text-emerald-500"
		: "flex items-center justify-between p-3 text-sm";
	const textClass = muted ? "line-through text-muted-foreground" : "";
	const prefix = additive ? "+ " : "";

	return (
		<div className={wrapperClass}>
			<span className={textClass}>
				{prefix}
				<strong>{displayObject(perm.object)}</strong> in{" "}
				<strong>{displayDomain(perm.domain)}</strong> —{" "}
				{formatAction(perm.action)}
			</span>
			{trailing}
		</div>
	);
}

// ---- AddPermissionForm ------------------------------------------------------

interface CatalogEntry {
	object: PermissionObject;
	scope: ObjectScope;
	actions: PermissionAction[];
}

interface AddPermissionFormProps {
	catalog: CatalogEntry[];
	catalogLoading: boolean;
	onStage: (perm: PermissionAssignment) => void;
}

function AddPermissionForm({
	catalog,
	catalogLoading,
	onStage,
}: Readonly<AddPermissionFormProps>) {
	const firstEntry = catalog[0];
	const [object, setObject] = useState<PermissionObject | undefined>(undefined);

	const entry = useMemo(
		() => catalog.find((e) => e.object === object) ?? firstEntry,
		[catalog, object, firstEntry],
	);

	const [action, setAction] = useState<PermissionAction | undefined>(undefined);
	const [domainAll, setDomainAll] = useState(true);
	const [domainSelection, setDomainSelection] = useState<string[]>([]);

	// When the catalog finishes loading (or the chosen object disappears), pin
	// state to the first entry. This also handles the initial render where
	// `object` is undefined.
	const effectiveObject = entry?.object;
	const effectiveAction = useMemo(() => {
		if (!entry) return undefined;
		if (action !== undefined && entry.actions.includes(action)) return action;
		return entry.actions[0];
	}, [entry, action]);

	if (catalogLoading || !entry || effectiveObject === undefined) {
		return (
			<div className="rounded-xl border bg-card p-4 text-sm text-muted-foreground">
				Loading catalog…
			</div>
		);
	}

	const handleObjectChange = (next: PermissionObject) => {
		setObject(next);
		setAction(undefined);
		setDomainAll(true);
		setDomainSelection([]);
	};

	const canStage =
		effectiveAction !== undefined &&
		(entry.scope === ObjectScope.GLOBAL ||
			domainAll ||
			domainSelection.length === 1);

	const handleStage = () => {
		if (effectiveAction === undefined) return;
		const domain = resolveDomain(entry.scope, domainAll, domainSelection);
		if (domain === null) return;
		onStage(
			create(PermissionAssignmentSchema, {
				object: effectiveObject,
				action: effectiveAction,
				domain,
			}),
		);
		// Reset domain choice but keep object/action — repeating grants is common.
		setDomainAll(true);
		setDomainSelection([]);
	};

	return (
		<div className="rounded-xl border bg-card p-4">
			<p className="text-sm font-medium mb-2">Add permission</p>
			<div className="flex flex-wrap gap-2 items-end">
				<div className="flex flex-col gap-1">
					<Label className="text-xs text-muted-foreground">Object</Label>
					<Select
						value={String(effectiveObject)}
						onValueChange={(v) =>
							handleObjectChange(Number(v) as PermissionObject)
						}
					>
						<SelectTrigger
							className="w-36 h-8 text-sm"
							aria-label="Permission object"
						>
							<SelectValue />
						</SelectTrigger>
						<SelectContent>
							{catalog.map((e) => (
								<SelectItem key={e.object} value={String(e.object)}>
									{displayObject(e.object)}
								</SelectItem>
							))}
						</SelectContent>
					</Select>
				</div>

				<div className="flex flex-col gap-1">
					<Label className="text-xs text-muted-foreground">Action</Label>
					<Select
						value={effectiveAction !== undefined ? String(effectiveAction) : ""}
						onValueChange={(v) => setAction(Number(v) as PermissionAction)}
					>
						<SelectTrigger
							className="w-28 h-8 text-sm"
							aria-label="Permission action"
						>
							<SelectValue />
						</SelectTrigger>
						<SelectContent>
							{entry.actions.map((a) => (
								<SelectItem key={a} value={String(a)}>
									{formatAction(a)}
								</SelectItem>
							))}
						</SelectContent>
					</Select>
				</div>

				<DomainPicker
					scope={entry.scope}
					action={effectiveAction}
					all={domainAll}
					onAllChange={(next) => {
						setDomainAll(next);
						if (next) setDomainSelection([]);
					}}
					selection={domainSelection}
					onSelectionChange={setDomainSelection}
				/>

				<Button
					type="button"
					size="sm"
					variant="outline"
					disabled={!canStage}
					onClick={handleStage}
				>
					Add
				</Button>
			</div>
		</div>
	);
}

// ---- DomainPicker -----------------------------------------------------------

interface DomainPickerProps {
	scope: ObjectScope;
	action: PermissionAction | undefined;
	all: boolean;
	onAllChange: (next: boolean) => void;
	selection: string[];
	onSelectionChange: (next: string[]) => void;
}

function DomainPicker({
	scope,
	action,
	all,
	onAllChange,
	selection,
	onSelectionChange,
}: Readonly<DomainPickerProps>) {
	const allId = useId();
	if (scope === ObjectScope.GLOBAL) {
		return null;
	}

	const pickerActions = action !== undefined ? [action] : [];
	const label = scope === ObjectScope.NAMESPACE ? "Namespace" : "Group";

	return (
		<div className="flex flex-col gap-1">
			<Label className="text-xs text-muted-foreground">{label}</Label>
			<div className="flex items-center gap-2">
				<div className="flex items-center gap-1 text-sm">
					<Checkbox
						id={allId}
						checked={all}
						onCheckedChange={(v) => onAllChange(v === true)}
					/>
					<Label htmlFor={allId} className="text-sm">
						All
					</Label>
				</div>
				<div className="w-56">
					{scope === ObjectScope.NAMESPACE ? (
						<NamespaceFilter
							value={selection}
							onValueChange={onSelectionChange}
							permissionActions={pickerActions}
							multiple={false}
							disabled={all}
						/>
					) : (
						<GroupFilter
							value={selection}
							onValueChange={onSelectionChange}
							permissionActions={pickerActions}
							multiple={false}
							disabled={all}
						/>
					)}
				</div>
			</div>
		</div>
	);
}

// ---- helpers ----------------------------------------------------------------

// resolveDomain encodes the canonical wire format the backend expects for a
// permission's domain field per scope. Returns null if the user has not
// completed a required selection — callers should not stage in that case.
function resolveDomain(
	scope: ObjectScope,
	all: boolean,
	selection: string[],
): string | null {
	if (scope === ObjectScope.GLOBAL) return WILDCARD_DOMAIN;
	if (all) return WILDCARD_DOMAIN;
	const picked = selection[0];
	if (!picked) return null;
	if (scope === ObjectScope.GROUP) return groupResource(picked);
	if (scope === ObjectScope.NAMESPACE) return namespaceResource(picked);
	return picked;
}

// displayDomain renders a stored Casbin domain in human form by stripping the
// canonical resource prefixes. The remaining id/name is shown as-is —
// resolving group ids to friendly names would require a per-row lookup that
// belongs further up the page.
function displayDomain(d: string): string {
	if (d === WILDCARD_DOMAIN) return "all";
	if (d.startsWith(NAMESPACE_DOMAIN_PREFIX)) {
		return d.slice(NAMESPACE_DOMAIN_PREFIX.length);
	}
	if (d.startsWith(GROUP_DOMAIN_PREFIX)) {
		return d.slice(GROUP_DOMAIN_PREFIX.length);
	}
	return d;
}

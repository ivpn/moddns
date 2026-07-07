import { useCallback, useEffect, useMemo, useState, type JSX, type ReactNode } from "react";
import {
    DndContext,
    DragOverlay,
    closestCorners,
    KeyboardSensor,
    PointerSensor,
    useDroppable,
    useSensor,
    useSensors,
    type DragEndEvent,
    type DragOverEvent,
    type DragStartEvent,
} from "@dnd-kit/core";
import {
    SortableContext,
    arrayMove,
    sortableKeyboardCoordinates,
    useSortable,
    verticalListSortingStrategy,
} from "@dnd-kit/sortable";
import { CSS } from "@dnd-kit/utilities";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogFooter,
    DialogHeader,
    DialogTitle,
} from "@/components/ui/dialog";
import {
    DropdownMenu,
    DropdownMenuContent,
    DropdownMenuItem,
    DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import {
    Check,
    Folder,
    FolderOpen,
    FolderPlus,
    GripVertical,
    MinusIcon,
    MoreVertical,
    Pencil,
    StickyNote,
    Trash2,
    X,
} from "lucide-react";
import type { ModelCustomRule } from "@/api/client/api";
import NoRulesExist from "@/pages/custom_rules/NoRulesExist";
import CustomRuleEntry from "@/pages/custom_rules/Entry";
import { useAppStore } from "@/store/general";

const UNGROUPED = "";
const CONTAINER_PREFIX = "group:";
const NEW_GROUP_ZONE = "group:__new__";
// A second droppable per group, anchored on the always-visible header, so a rule
// can be dropped onto a group even when its body is collapsed (QA #634). It maps
// to the same group as the body container but needs a distinct id (dnd-kit ids
// must be unique).
const HEADER_PREFIX = "grouphdr:";

const containerId = (group: string) => `${CONTAINER_PREFIX}${group}`;
const isContainerId = (id: string) => id.startsWith(CONTAINER_PREFIX);
const groupFromContainer = (id: string) => id.slice(CONTAINER_PREFIX.length);
const headerDropId = (group: string) => `${HEADER_PREFIX}${group}`;
const isHeaderDropId = (id: string) => id.startsWith(HEADER_PREFIX);
const groupFromHeader = (id: string) => id.slice(HEADER_PREFIX.length);
// A drop target that resolves to a group: either the body container or the header.
const isGroupDropId = (id: string) => isContainerId(id) || isHeaderDropId(id);
const groupFromDropId = (id: string) =>
    isHeaderDropId(id) ? groupFromHeader(id) : groupFromContainer(id);

// Group headers are reorderable via their own sortable layer. Their sortable ids use
// a distinct prefix so they never collide with rule ids or the `group:` droppable /
// NEW_GROUP_ZONE container ids shared by the rule-drag layer.
const GROUP_SORT_PREFIX = "groupsort:";
const groupSortId = (name: string) => `${GROUP_SORT_PREFIX}${name}`;
const isGroupSortId = (id: string) => id.startsWith(GROUP_SORT_PREFIX);
const groupFromSortId = (id: string) => id.slice(GROUP_SORT_PREFIX.length);

export interface CustomRulesCardProps {
    rules: ModelCustomRule[];
    groupNotes: Record<string, string>;
    // Ordered group names from the per-list registry (custom_rule_groups). Drives the
    // display order of group sections; groups discovered only via a rule label are
    // appended after these.
    groupOrder: string[];
    selectedIds: string[];
    onCheck: (id: string, checked: boolean) => void;
    onDelete: (id: string) => void | Promise<void>;
    onEdit: (rule: ModelCustomRule) => void;
    onReorder: (orderedIds: string[]) => void | Promise<void>;
    onMoveRule: (orderedIds: string[], ruleId: string, newGroup: string) => void | Promise<void>;
    onSaveGroupNote: (group: string, note: string | null) => void | Promise<void>;
    onCreateGroup: (name: string) => void | Promise<void>;
    onRenameGroup: (from: string, to: string) => void | Promise<void>;
    onDeleteGroup: (name: string) => void | Promise<void>;
    onReorderGroups: (orderedNames: string[]) => void | Promise<void>;
    allSelected: boolean;
    selectedCount: number;
    handleBulkDelete: () => void | Promise<void>;
    loading: boolean;
    type: "denied" | "allowed";
    searchQuery: string;
}

// ── Row ──────────────────────────────────────────────────────────────────────
// A sortable rule row. The grip handle stays hidden until the row is hovered so
// the list reads clean at rest, then invites the drag on approach.
function SortableEntry({
    rule,
    checked,
    onCheck,
    onDelete,
    onEdit,
    isRemoving,
    hideDeleteButton,
    draggable,
}: {
    rule: ModelCustomRule;
    checked: boolean;
    onCheck: (id: string, checked: boolean) => void;
    onDelete: (id: string) => void;
    onEdit: (rule: ModelCustomRule) => void;
    isRemoving: boolean;
    hideDeleteButton: boolean;
    draggable: boolean;
}): JSX.Element {
    const { attributes, listeners, setNodeRef, transform, transition, isDragging } = useSortable({
        id: rule.id,
        disabled: !draggable,
    });

    const style: React.CSSProperties = {
        transform: CSS.Transform.toString(transform),
        transition,
        // The original row becomes a faint placeholder while its overlay clone is dragged.
        opacity: isDragging ? 0.35 : 1,
    };

    const handle = draggable ? (
        <button
            type="button"
            className="flex items-center justify-center cursor-grab active:cursor-grabbing touch-none text-[var(--tailwind-colors-slate-500)] hover:text-[var(--tailwind-colors-slate-200)] opacity-0 group-hover/row:opacity-100 focus-visible:opacity-100 transition-opacity shrink-0 -ml-1"
            aria-label="Drag to reorder or move between groups"
            {...attributes}
            {...listeners}
        >
            <GripVertical className="w-4 h-4" />
        </button>
    ) : undefined;

    return (
        <div ref={setNodeRef} style={style} className="w-full group/row">
            <CustomRuleEntry
                rule={rule}
                checked={checked}
                onCheck={onCheck}
                onDelete={onDelete}
                onEdit={onEdit}
                isRemoving={isRemoving}
                hideDeleteButton={hideDeleteButton}
                dragHandle={handle}
            />
        </div>
    );
}

// ── Droppable section body ─────────────────────────────────────────────────────
// Wraps a group's rows so empty/collapsed groups still accept a drop, and lights
// up in the accent colour the instant a dragged row is over it.
function DroppableSection({
    group,
    isEmpty,
    children,
}: {
    group: string;
    isEmpty: boolean;
    children: ReactNode;
}): JSX.Element {
    const { setNodeRef, isOver } = useDroppable({ id: containerId(group) });

    return (
        <div
            ref={setNodeRef}
            className={[
                "flex flex-col w-full rounded-md transition-colors",
                isEmpty
                    ? "min-h-16 items-center justify-center border border-dashed text-xs"
                    : "items-start gap-2",
                isOver
                    ? "border-[var(--tailwind-colors-rdns-600)] bg-[var(--tailwind-colors-rdns-600)]/5"
                    : isEmpty
                        ? "border-[var(--tailwind-colors-slate-light-400)] text-[var(--tailwind-colors-slate-light-600)] dark:border-[var(--tailwind-colors-slate-700)] dark:text-[var(--tailwind-colors-slate-500)]"
                        : "",
            ].join(" ")}
        >
            {isEmpty ? <span>Drop rules here</span> : children}
        </div>
    );
}

// ── Group header ───────────────────────────────────────────────────────────────
function GroupHeader({
    name,
    count,
    collapsed,
    onToggle,
    note,
    onSaveNote,
    onRename,
    onDelete,
    loading,
    dragHandle,
    groupDragActive,
}: {
    name: string;
    count: number;
    collapsed: boolean;
    onToggle: () => void;
    note: string;
    onSaveNote: (note: string | null) => void;
    onRename: (to: string) => void;
    onDelete: () => void;
    loading: boolean;
    dragHandle?: JSX.Element | null;
    groupDragActive: boolean;
}): JSX.Element {
    const [editingNote, setEditingNote] = useState(false);
    const [noteDraft, setNoteDraft] = useState(note);
    const [renaming, setRenaming] = useState(false);
    const [nameDraft, setNameDraft] = useState(name);

    // Header doubles as a drop target so a rule can be dropped onto a group even
    // when its body is collapsed (QA #634) — the collapsed body has zero height and
    // is not hittable, so without this a collapsed group could not receive a drop.
    // The destination is expanded after the drop (see handleDragEnd) rather than on
    // hover, to avoid shifting the layout mid-drag.
    // Disabled while a group is being dragged: this header droppable would otherwise
    // compete with the group-reorder sortable for the same region and steal the
    // `over` target, silently breaking group reordering.
    const { setNodeRef, isOver } = useDroppable({ id: headerDropId(name), disabled: groupDragActive });

    useEffect(() => { setNoteDraft(note); }, [note]);
    useEffect(() => { setNameDraft(name); }, [name]);

    const commitNote = () => { onSaveNote(noteDraft.trim() === "" ? null : noteDraft.trim()); setEditingNote(false); };
    const cancelNote = () => { setNoteDraft(note); setEditingNote(false); };
    const commitName = () => {
        const next = nameDraft.trim();
        if (next && next !== name) onRename(next);
        setRenaming(false);
    };
    const cancelName = () => { setNameDraft(name); setRenaming(false); };

    if (renaming) {
        return (
            <div className="flex items-center gap-1 w-full px-1 py-1.5 mt-2">
                <Input
                    value={nameDraft}
                    onChange={(e) => setNameDraft(e.target.value.slice(0, 64))}
                    onKeyDown={(e) => {
                        if (e.key === "Enter") { e.preventDefault(); commitName(); }
                        else if (e.key === "Escape") { e.preventDefault(); cancelName(); }
                    }}
                    className="h-10 md:h-7 w-56 max-w-full"
                    autoFocus
                />
                <Button variant="ghost" size="sm" className="h-10 w-10 md:h-7 md:w-7 p-0 shrink-0" disabled={loading} aria-label="Save group name" onClick={commitName}>
                    <Check className="w-5 h-5 md:w-4 md:h-4 text-[var(--tailwind-colors-rdns-600)]" />
                </Button>
                <Button variant="ghost" size="sm" className="h-10 w-10 md:h-7 md:w-7 p-0 shrink-0" aria-label="Cancel rename" onClick={cancelName}>
                    <X className="w-5 h-5 md:w-4 md:h-4 text-[var(--tailwind-colors-slate-400)]" />
                </Button>
            </div>
        );
    }

    return (
        <div
            ref={setNodeRef}
            className={[
                "flex flex-col w-full px-1 py-1.5 mt-2 gap-0.5 rounded-md transition-colors",
                isOver ? "bg-[var(--tailwind-colors-rdns-600)]/10 ring-1 ring-[var(--tailwind-colors-rdns-600)]/40" : "",
            ].join(" ")}
        >
            <div className="flex items-center gap-1 w-full">
                {dragHandle}
                <button
                    type="button"
                    onClick={onToggle}
                    className="flex items-center gap-3 md:gap-2 min-w-0 py-1.5 md:py-0 text-[var(--tailwind-colors-slate-100)] cursor-pointer"
                >
                    {collapsed ? <Folder className="w-5 h-5 md:w-4 md:h-4 shrink-0" /> : <FolderOpen className="w-5 h-5 md:w-4 md:h-4 shrink-0" />}
                    <span className="font-medium text-base md:text-sm truncate min-w-0">{name}</span>
                    <span className="text-sm md:text-xs text-[var(--tailwind-colors-slate-400)] shrink-0">{count}</span>
                </button>

                {editingNote ? (
                    <div className="flex items-center gap-1 min-w-0">
                        <Input
                            value={noteDraft}
                            onChange={(e) => setNoteDraft(e.target.value.slice(0, 80))}
                            onKeyDown={(e) => {
                                if (e.key === "Enter") { e.preventDefault(); commitNote(); }
                                else if (e.key === "Escape") { e.preventDefault(); cancelNote(); }
                            }}
                            placeholder="Group note"
                            className="h-10 md:h-7 w-48 max-w-full"
                            autoFocus
                        />
                        <Button variant="ghost" size="sm" className="h-10 w-10 md:h-7 md:w-7 p-0 shrink-0" disabled={loading} aria-label="Save group note" onClick={commitNote}>
                            <Check className="w-5 h-5 md:w-4 md:h-4 text-[var(--tailwind-colors-rdns-600)]" />
                        </Button>
                        <Button variant="ghost" size="sm" className="h-10 w-10 md:h-7 md:w-7 p-0 shrink-0" aria-label="Cancel" onClick={cancelNote}>
                            <X className="w-5 h-5 md:w-4 md:h-4 text-[var(--tailwind-colors-slate-400)]" />
                        </Button>
                    </div>
                ) : (
                    /* modal={false} so Radix never sets pointer-events:none on <body>.
                       The Delete item opens a confirm Dialog whose confirmation refetches
                       the profile and unmounts this header (and this menu). A modal menu
                       unmounted mid-close never runs its body-unlock cleanup, leaving the
                       whole app frozen. Non-modal has no body lock, so the freeze cannot occur. */
                    <DropdownMenu modal={false}>
                        <DropdownMenuTrigger asChild>
                            <Button variant="ghost" size="sm" className="h-10 w-10 md:h-7 md:w-7 p-0 shrink-0" aria-label="Group actions">
                                <MoreVertical className="w-5 h-5 md:w-3.5 md:h-3.5 text-[var(--tailwind-colors-slate-400)]" />
                            </Button>
                        </DropdownMenuTrigger>
                        <DropdownMenuContent side="top" align="start">
                            <DropdownMenuItem onClick={() => setRenaming(true)}>
                                <Pencil className="w-4 h-4 mr-2" /> Rename group
                            </DropdownMenuItem>
                            <DropdownMenuItem onClick={() => setEditingNote(true)}>
                                <StickyNote className="w-4 h-4 mr-2" /> {note ? "Edit comment" : "Add comment"}
                            </DropdownMenuItem>
                            <DropdownMenuItem
                                className="text-[var(--tailwind-colors-red-600)] focus:text-[var(--tailwind-colors-red-600)] dark:text-[var(--tailwind-colors-red-400)] dark:focus:text-[var(--tailwind-colors-red-400)]"
                                onClick={onDelete}
                            >
                                <Trash2 className="w-4 h-4 mr-2 text-[var(--tailwind-colors-red-600)] dark:text-[var(--tailwind-colors-red-400)]" /> Delete group
                            </DropdownMenuItem>
                        </DropdownMenuContent>
                    </DropdownMenu>
                )}
            </div>

            {/* Group comment as an always-visible muted line under the name (indented
              to align past the folder icon). Real text — no hover/tooltip. */}
            {note && !editingNote && (
                <div className="pl-8 md:pl-6 text-xs leading-4 line-clamp-2 text-[var(--tailwind-colors-slate-light-600)] dark:text-[var(--tailwind-colors-slate-400)]">
                    {note}
                </div>
            )}
        </div>
    );
}

// ── Create-group zone (button + create-on-drop target) ─────────────────────────
function NewGroupZone({
    creating,
    onStartCreate,
    onCancelCreate,
    onConfirmCreate,
    namingForDrop,
}: {
    creating: boolean;
    onStartCreate: () => void;
    onCancelCreate: () => void;
    onConfirmCreate: (name: string) => void;
    namingForDrop: boolean;
}): JSX.Element {
    const { setNodeRef, isOver } = useDroppable({ id: NEW_GROUP_ZONE });
    const [draft, setDraft] = useState("");

    useEffect(() => { if (!creating) setDraft(""); }, [creating]);

    if (creating) {
        return (
            <div className="flex items-center gap-1 w-full px-1 py-1.5">
                <FolderPlus className="w-4 h-4 text-[var(--tailwind-colors-rdns-600)] shrink-0" />
                <Input
                    value={draft}
                    onChange={(e) => setDraft(e.target.value.slice(0, 64))}
                    onKeyDown={(e) => {
                        if (e.key === "Enter") { e.preventDefault(); if (draft.trim()) onConfirmCreate(draft.trim()); }
                        else if (e.key === "Escape") { e.preventDefault(); onCancelCreate(); }
                    }}
                    placeholder={namingForDrop ? "Name the new group for this rule…" : "New group name…"}
                    className="h-10 md:h-7 w-56 max-w-full"
                    autoFocus
                />
                <Button variant="ghost" size="sm" className="h-10 w-10 md:h-7 md:w-7 p-0 shrink-0" aria-label="Create group" disabled={!draft.trim()} onClick={() => draft.trim() && onConfirmCreate(draft.trim())}>
                    <Check className="w-5 h-5 md:w-4 md:h-4 text-[var(--tailwind-colors-rdns-600)]" />
                </Button>
                <Button variant="ghost" size="sm" className="h-10 w-10 md:h-7 md:w-7 p-0 shrink-0" aria-label="Cancel" onClick={onCancelCreate}>
                    <X className="w-5 h-5 md:w-4 md:h-4 text-[var(--tailwind-colors-slate-400)]" />
                </Button>
            </div>
        );
    }

    return (
        <button
            ref={setNodeRef}
            type="button"
            onClick={onStartCreate}
            className={[
                // Dashed border keeps the "drop a rule here" affordance; the accent
                // fill + colour is what sets it apart from the plain "Drop rules here" zone.
                "flex items-center justify-center gap-2 w-full min-h-16 mt-1 rounded-md border border-dashed text-sm font-medium transition-colors cursor-pointer",
                isOver
                    // Brighter while a rule is dragged over — the active drop target.
                    ? "border-[var(--tailwind-colors-rdns-600)] bg-[var(--tailwind-colors-rdns-600)]/10 text-[var(--tailwind-colors-rdns-600)]"
                    : "border-[var(--tailwind-colors-rdns-600)]/40 bg-[var(--tailwind-colors-rdns-600)]/5 text-[var(--tailwind-colors-rdns-600)] hover:bg-[var(--tailwind-colors-rdns-600)]/10 hover:border-[var(--tailwind-colors-rdns-600)]",
            ].join(" ")}
        >
            <FolderPlus className="w-4 h-4" />
            {isOver ? "Drop to create a new group" : "New group"}
        </button>
    );
}

// ── Section body ───────────────────────────────────────────────────────────────
// The collapse-animated list of a section's rules (its own rule-level SortableContext
// + droppable). Shared by the Ungrouped section and every named group.
function SectionBody({
    section,
    isCollapsed,
    selectedIds,
    onCheck,
    onEntryDelete,
    onEdit,
    removingIds,
    allSelected,
    draggable,
}: {
    section: { name: string; items: ModelCustomRule[] };
    isCollapsed: boolean;
    selectedIds: string[];
    onCheck: (id: string, checked: boolean) => void;
    onEntryDelete: (id: string) => void;
    onEdit: (rule: ModelCustomRule) => void;
    removingIds: string[];
    allSelected: boolean;
    draggable: boolean;
}): JSX.Element {
    const isEmpty = section.items.length === 0;
    const sectionIds = section.items.map(r => r.id);
    return (
        /* grid-template-rows 0fr<->1fr animates the section height fluidly without
           measuring; the inner wrapper clips during the transition. Kept mounted so
           it stays a drop target. */
        <div className={`grid transition-[grid-template-rows] duration-300 ease-in-out motion-reduce:transition-none ${isCollapsed ? "grid-rows-[0fr]" : "grid-rows-[1fr]"}`}>
            <div className={`min-h-0 overflow-hidden transition-opacity duration-200 ${isCollapsed ? "opacity-0" : "opacity-100"}`}>
                <SortableContext items={sectionIds} strategy={verticalListSortingStrategy}>
                    <DroppableSection group={section.name} isEmpty={isEmpty}>
                        {section.items.map((rule) => (
                            <SortableEntry
                                key={rule.id}
                                rule={rule}
                                checked={selectedIds.includes(rule.id)}
                                onCheck={onCheck}
                                onDelete={onEntryDelete}
                                onEdit={onEdit}
                                isRemoving={removingIds.includes(rule.id)}
                                hideDeleteButton={allSelected}
                                draggable={draggable}
                            />
                        ))}
                    </DroppableSection>
                </SortableContext>
            </div>
        </div>
    );
}

// ── Sortable group section ─────────────────────────────────────────────────────
// A named group whose header carries a grip handle to reorder the whole section.
// The group-level useSortable lives here (must be inside the group SortableContext),
// while the rule-level SortableContext for its rows lives in SectionBody.
function SortableGroupSection({
    section,
    collapsed,
    onToggleCollapse,
    note,
    onSaveNote,
    onRename,
    onRequestDelete,
    loading,
    draggable,
    groupDragActive,
    selectedIds,
    onCheck,
    onEntryDelete,
    onEdit,
    removingIds,
    allSelected,
}: {
    section: { name: string; items: ModelCustomRule[] };
    collapsed: boolean;
    onToggleCollapse: () => void;
    note: string;
    onSaveNote: (note: string | null) => void;
    onRename: (to: string) => void;
    onRequestDelete: () => void;
    loading: boolean;
    draggable: boolean;
    groupDragActive: boolean;
    selectedIds: string[];
    onCheck: (id: string, checked: boolean) => void;
    onEntryDelete: (id: string) => void;
    onEdit: (rule: ModelCustomRule) => void;
    removingIds: string[];
    allSelected: boolean;
}): JSX.Element {
    const { setNodeRef, attributes, listeners, transform, transition, isDragging } = useSortable({
        id: groupSortId(section.name),
        disabled: !draggable,
    });
    const style: React.CSSProperties = {
        transform: CSS.Transform.toString(transform),
        transition,
        opacity: isDragging ? 0.4 : 1,
    };
    const handle = draggable ? (
        <button
            type="button"
            className="flex items-center justify-center cursor-grab active:cursor-grabbing touch-none text-[var(--tailwind-colors-slate-500)] hover:text-[var(--tailwind-colors-slate-200)] shrink-0 -ml-1"
            aria-label={`Drag to reorder group ${section.name}`}
            {...attributes}
            {...listeners}
        >
            <GripVertical className="w-4 h-4" />
        </button>
    ) : null;
    return (
        <div ref={setNodeRef} style={style} className="w-full">
            <GroupHeader
                name={section.name}
                count={section.items.length}
                collapsed={collapsed}
                onToggle={onToggleCollapse}
                note={note}
                onSaveNote={onSaveNote}
                onRename={onRename}
                onDelete={onRequestDelete}
                loading={loading}
                dragHandle={handle}
                groupDragActive={groupDragActive}
            />
            <SectionBody
                section={section}
                isCollapsed={collapsed}
                selectedIds={selectedIds}
                onCheck={onCheck}
                onEntryDelete={onEntryDelete}
                onEdit={onEdit}
                removingIds={removingIds}
                allSelected={allSelected}
                draggable={draggable}
            />
        </div>
    );
}

export default function CustomRulesCard({
    rules,
    groupNotes,
    groupOrder,
    selectedIds,
    onCheck,
    onDelete,
    onEdit,
    onReorder,
    onMoveRule,
    onSaveGroupNote,
    onCreateGroup,
    onRenameGroup,
    onDeleteGroup,
    onReorderGroups,
    allSelected,
    selectedCount,
    handleBulkDelete,
    loading,
    type,
    searchQuery,
}: CustomRulesCardProps): JSX.Element {
    const [removingIds, setRemovingIds] = useState<string[]>([]);
    // Collapsed group folders are persisted per-device in the app store, keyed by
    // profile + list type, so open/closed state survives navigation and reload.
    const activeProfile = useAppStore((s) => s.activeProfile);
    const profileId = (activeProfile as (typeof activeProfile) & { profile_id?: string })?.profile_id
        ?? activeProfile?.id ?? "";
    const storageKey = `${profileId}:${type}`;
    const persistedCollapsed = useAppStore((s) => s.customRulesCollapsed[storageKey]);
    const setCustomRulesCollapsed = useAppStore((s) => s.setCustomRulesCollapsed);
    const collapsed = useMemo(() => new Set(persistedCollapsed ?? []), [persistedCollapsed]);
    const updateCollapsed = useCallback(
        (updater: (prev: Set<string>) => Set<string>) => {
            const next = updater(new Set(persistedCollapsed ?? []));
            setCustomRulesCollapsed(storageKey, [...next]);
        },
        [persistedCollapsed, storageKey, setCustomRulesCollapsed],
    );
    const [activeId, setActiveId] = useState<string | null>(null);
    // The group name currently being dragged (group-reorder layer), null when a rule
    // (or nothing) is being dragged.
    const [activeGroupName, setActiveGroupName] = useState<string | null>(null);
    const [creatingGroup, setCreatingGroup] = useState(false);
    // When a row is dropped on the "new group" zone we hold its id and prompt for a name.
    const [pendingDropRuleId, setPendingDropRuleId] = useState<string | null>(null);
    // Group pending deletion confirmation (styled dialog instead of window.confirm).
    const [groupToDelete, setGroupToDelete] = useState<string | null>(null);

    const sortedFromProps = useMemo(
        () => [...rules].sort((a, b) => (a.order ?? 0) - (b.order ?? 0)),
        [rules],
    );
    const [localRules, setLocalRules] = useState<ModelCustomRule[]>(sortedFromProps);
    useEffect(() => { setLocalRules(sortedFromProps); }, [sortedFromProps]);

    const handleEntryDelete = useCallback((id: string) => {
        setRemovingIds(prev => [...prev, id]);
        setTimeout(() => {
            onDelete(id);
            setRemovingIds(prev => prev.filter(rid => rid !== id));
        }, 300);
    }, [onDelete]);

    const sensors = useSensors(
        useSensor(PointerSensor, { activationConstraint: { distance: 5 } }),
        useSensor(KeyboardSensor, { coordinateGetter: sortableKeyboardCoordinates }),
    );

    // Group display order = the registry order (groupOrder) first, then any group that
    // exists only via a rule's label but isn't in the registry yet, appended
    // alphabetically. Replaces the old purely-alphabetical ordering.
    const derivedGroupNames = useMemo(() => {
        const ordered: string[] = [];
        const seen = new Set<string>();
        for (const n of groupOrder) {
            if (n !== "" && !seen.has(n)) { ordered.push(n); seen.add(n); }
        }
        const labelOnly = new Set<string>();
        for (const r of localRules) {
            const g = r.group ?? "";
            if (g !== "" && !seen.has(g)) labelOnly.add(g);
        }
        const extras = Array.from(labelOnly).sort((a, b) => a.localeCompare(b, undefined, { sensitivity: "base" }));
        return [...ordered, ...extras];
    }, [localRules, groupOrder]);

    // Local, optimistic copy so a group drag reorders instantly; reset when the derived
    // order changes (i.e. after the persisted profile refetches). Mirrors localRules.
    const [localGroupOrder, setLocalGroupOrder] = useState<string[]>(derivedGroupNames);
    useEffect(() => { setLocalGroupOrder(derivedGroupNames); }, [derivedGroupNames]);
    const groupNames = localGroupOrder;

    // Sections: Ungrouped first, then named groups in stored order. Empty groups included.
    const sections = useMemo(() => {
        const byGroup = new Map<string, ModelCustomRule[]>();
        byGroup.set(UNGROUPED, []);
        for (const g of groupNames) byGroup.set(g, []);
        for (const r of localRules) {
            const g = r.group ?? UNGROUPED;
            if (!byGroup.has(g)) byGroup.set(g, []);
            byGroup.get(g)!.push(r);
        }
        const ordered = [{ name: UNGROUPED, items: byGroup.get(UNGROUPED)! }];
        for (const g of groupNames) ordered.push({ name: g, items: byGroup.get(g)! });
        return ordered;
    }, [localRules, groupNames]);

    const groupOf = useCallback(
        (id: string) => localRules.find(r => r.id === id)?.group ?? UNGROUPED,
        [localRules],
    );

    const draggable = searchQuery.trim().length === 0 && !allSelected && !loading;

    const handleDragStart = useCallback((event: DragStartEvent) => {
        const id = String(event.active.id);
        if (isGroupSortId(id)) { setActiveGroupName(groupFromSortId(id)); return; }
        setActiveId(id);
    }, []);

    // Live cross-section move: when the row hovers a different group, relabel it and
    // splice it into that group so the UI reflects the move mid-drag.
    const handleDragOver = useCallback((event: DragOverEvent) => {
        const { active, over } = event;
        if (!over) return;
        const activeKey = String(active.id);
        // Group-reorder drags are finalised on drop; no live rule relabelling.
        if (isGroupSortId(activeKey)) return;
        const overKey = String(over.id);
        if (overKey === NEW_GROUP_ZONE) return; // resolved on drop

        const targetGroup = isGroupDropId(overKey) ? groupFromDropId(overKey) : groupOf(overKey);
        const activeGroup = groupOf(activeKey);
        if (targetGroup === activeGroup) return;

        setLocalRules(prev => {
            const activeIdx = prev.findIndex(r => r.id === activeKey);
            if (activeIdx < 0) return prev;
            const next = [...prev];
            const [moved] = next.splice(activeIdx, 1);
            const relabelled = { ...moved, group: targetGroup === UNGROUPED ? "" : targetGroup };

            let insertIdx: number;
            if (isGroupDropId(overKey)) {
                // Append to the end of the target group.
                let last = -1;
                next.forEach((r, i) => { if ((r.group ?? "") === (targetGroup === UNGROUPED ? "" : targetGroup)) last = i; });
                insertIdx = last + 1;
            } else {
                insertIdx = next.findIndex(r => r.id === overKey);
                if (insertIdx < 0) insertIdx = next.length;
            }
            next.splice(insertIdx, 0, relabelled);
            return next;
        });
    }, [groupOf]);

    const handleDragEnd = useCallback((event: DragEndEvent) => {
        const { active, over } = event;
        const activeKey = String(active.id);

        // Group-reorder layer: reorder the group sections and persist the new order.
        if (isGroupSortId(activeKey)) {
            setActiveGroupName(null);
            if (!over) return;
            const overKey = String(over.id);
            if (!isGroupSortId(overKey)) return;
            const from = groupFromSortId(activeKey);
            const to = groupFromSortId(overKey);
            if (from === to) return;
            const oldIndex = localGroupOrder.indexOf(from);
            const newIndex = localGroupOrder.indexOf(to);
            if (oldIndex < 0 || newIndex < 0 || oldIndex === newIndex) return;
            const nextOrder = arrayMove(localGroupOrder, oldIndex, newIndex);
            setLocalGroupOrder(nextOrder);
            void onReorderGroups(nextOrder);
            return;
        }

        setActiveId(null);

        if (!over) { setLocalRules(sortedFromProps); return; }
        const overKey = String(over.id);

        // Dropped on the create-group zone: revert the visual and prompt for a name.
        if (overKey === NEW_GROUP_ZONE) {
            setLocalRules(sortedFromProps);
            setPendingDropRuleId(activeKey);
            setCreatingGroup(true);
            return;
        }

        // Same-list reorder finalisation.
        let next = localRules;
        if (!isGroupDropId(overKey) && activeKey !== overKey) {
            const oldIndex = localRules.findIndex(r => r.id === activeKey);
            const newIndex = localRules.findIndex(r => r.id === overKey);
            if (oldIndex >= 0 && newIndex >= 0 && oldIndex !== newIndex) {
                next = arrayMove(localRules, oldIndex, newIndex);
                setLocalRules(next);
            }
        }

        const originalGroup = rules.find(r => r.id === activeKey)?.group ?? "";
        const finalGroup = next.find(r => r.id === activeKey)?.group ?? "";
        const orderedIds = next.map(r => r.id);

        if (finalGroup !== originalGroup) {
            // Reveal the destination if it was collapsed, so the moved rule is
            // visible after a drop onto a collapsed group header.
            if (finalGroup !== UNGROUPED) {
                updateCollapsed(prev => {
                    if (!prev.has(finalGroup)) return prev;
                    const next = new Set(prev);
                    next.delete(finalGroup);
                    return next;
                });
            }
            void onMoveRule(orderedIds, activeKey, finalGroup);
        } else {
            // Only persist if order actually changed.
            const prevIds = sortedFromProps.map(r => r.id);
            if (orderedIds.join("|") !== prevIds.join("|")) void onReorder(orderedIds);
        }
    }, [localRules, localGroupOrder, rules, sortedFromProps, onMoveRule, onReorder, onReorderGroups, updateCollapsed]);

    const confirmNewGroup = useCallback((name: string) => {
        if (pendingDropRuleId) {
            // Create-on-drop: assigning the rule's group makes the group exist.
            const orderedIds = localRules.map(r => r.id);
            void onMoveRule(orderedIds, pendingDropRuleId, name);
            setPendingDropRuleId(null);
        } else {
            void onCreateGroup(name);
        }
        setCreatingGroup(false);
    }, [pendingDropRuleId, localRules, onMoveRule, onCreateGroup]);

    const cancelNewGroup = useCallback(() => {
        setCreatingGroup(false);
        setPendingDropRuleId(null);
    }, []);

    const confirmDeleteGroup = useCallback(() => {
        if (groupToDelete !== null) void onDeleteGroup(groupToDelete);
        setGroupToDelete(null);
    }, [groupToDelete, onDeleteGroup]);

    const toggleCollapse = useCallback((name: string) => {
        updateCollapsed(prev => {
            const next = new Set(prev);
            if (next.has(name)) next.delete(name);
            else next.add(name);
            return next;
        });
    }, [updateCollapsed]);

    const activeRule = activeId ? localRules.find(r => r.id === activeId) : null;
    const hasNamedGroups = groupNames.length > 0;
    // sections[0] is always Ungrouped; the rest are named groups in stored order.
    const ungroupedSection = sections[0];
    const namedSections = sections.slice(1);
    const groupSortIds = namedSections.map(s => groupSortId(s.name));
    const activeGroupSection = activeGroupName !== null
        ? namedSections.find(s => s.name === activeGroupName)
        : undefined;

    if (rules.length === 0) {
        if (searchQuery.trim().length > 0) {
            return (
                <Card className="flex flex-1 h-full items-center justify-center border-[var(--tailwind-colors-slate-600)] rounded-md bg-background">
                    <NoRulesExist
                        type={type}
                        title="No results found"
                        message={`Try a different search term or clear your search to see all ${type === "denied" ? "denylist" : "allowlist"} domains.`}
                    />
                </Card>
            );
        }

        return (
            <Card className="flex flex-col flex-1 self-stretch w-full grow bg-transparent dark:bg-[var(--variable-collection-surface)] rounded-lg border border-[var(--tailwind-colors-slate-light-300)] dark:border-transparent">
                <div className="flex flex-col items-center gap-6 p-6 w-full text-center">
                    <NoRulesExist
                        type={type}
                        title={type === "allowed" ? "There are no allowed domains yet." : undefined}
                    />
                </div>
            </Card>
        );
    }

    return (
        <div className="flex flex-col items-start gap-2 relative flex-1 self-stretch w-full grow rounded-md">
            {allSelected && selectedCount > 0 && (
                <div className="flex justify-between py-2 px-4 w-full bg-[var(--tailwind-colors-slate-900)] border border-solid border-[var(--tailwind-colors-slate-600)] rounded-md items-center">
                    <div className="inline-flex items-center gap-4">
                        <div className="relative w-4 h-4 bg-[var(--tailwind-colors-rdns-600)] rounded-[var(--shadcn-ui-radius-radius-sm)] border-[var(--tailwind-colors-rdns-600)]">
                            <MinusIcon className="absolute w-3.5 h-3.5 top-px left-px" />
                        </div>
                        <div className="font-text-sm-leading-5-normal text-[var(--tailwind-colors-slate-50)] text-[length:var(--text-sm-leading-5-normal-font-size)] tracking-[var(--text-sm-leading-5-normal-letter-spacing)] leading-[var(--text-sm-leading-5-normal-line-height)]">
                            {selectedCount} selected
                        </div>
                        <button
                            className="flex w-10 h-10 items-center justify-center rounded-[var(--primitives-radius-radius-md)] hover:bg-[var(--tailwind-colors-rdns-600)] group"
                            onClick={handleBulkDelete}
                            disabled={loading}
                            title="Delete selected entries"
                            aria-label="Delete selected entries"
                        >
                            <Trash2 className="w-4 h-4 text-[var(--tailwind-colors-rdns-600)] group-hover:text-[var(--tailwind-colors-slate-900)] transition-colors" />
                        </button>
                    </div>
                </div>
            )}

            <DndContext
                sensors={sensors}
                collisionDetection={closestCorners}
                onDragStart={handleDragStart}
                onDragOver={handleDragOver}
                onDragEnd={handleDragEnd}
            >
                <div className="flex flex-col items-start gap-2 w-full">
                    {/* Ungrouped section — pinned first, never part of the group-reorder
                        layer. Hidden when empty unless named groups exist (so it can serve
                        as a "remove from group" drop target). */}
                    {!(ungroupedSection.items.length === 0 && !hasNamedGroups) && (
                        <div className="w-full">
                            {hasNamedGroups && (
                                <div className="px-1 py-1.5 mt-2 text-xs font-medium text-[var(--tailwind-colors-slate-400)] uppercase tracking-wide">
                                    Ungrouped
                                </div>
                            )}
                            <SectionBody
                                section={ungroupedSection}
                                isCollapsed={collapsed.has(UNGROUPED)}
                                selectedIds={selectedIds}
                                onCheck={onCheck}
                                onEntryDelete={handleEntryDelete}
                                onEdit={onEdit}
                                removingIds={removingIds}
                                allSelected={allSelected}
                                draggable={draggable}
                            />
                        </div>
                    )}

                    {/* Named groups — reorderable via the grip handle on each header. */}
                    <SortableContext items={groupSortIds} strategy={verticalListSortingStrategy}>
                        {namedSections.map((section) => (
                            <SortableGroupSection
                                key={section.name}
                                section={section}
                                collapsed={collapsed.has(section.name)}
                                onToggleCollapse={() => toggleCollapse(section.name)}
                                note={groupNotes[section.name] ?? ""}
                                onSaveNote={(n) => onSaveGroupNote(section.name, n)}
                                onRename={(to) => onRenameGroup(section.name, to)}
                                onRequestDelete={() => setGroupToDelete(section.name)}
                                loading={loading}
                                draggable={draggable}
                                groupDragActive={activeGroupName !== null}
                                selectedIds={selectedIds}
                                onCheck={onCheck}
                                onEntryDelete={handleEntryDelete}
                                onEdit={onEdit}
                                removingIds={removingIds}
                                allSelected={allSelected}
                            />
                        ))}
                    </SortableContext>

                    {draggable && (
                        <NewGroupZone
                            creating={creatingGroup}
                            namingForDrop={pendingDropRuleId !== null}
                            onStartCreate={() => setCreatingGroup(true)}
                            onCancelCreate={cancelNewGroup}
                            onConfirmCreate={confirmNewGroup}
                        />
                    )}
                </div>

                <DragOverlay>
                    {activeRule ? (
                        <div className="w-full rounded-md ring-1 ring-[var(--tailwind-colors-rdns-600)] shadow-lg shadow-black/30 scale-[1.02] cursor-grabbing">
                            <CustomRuleEntry
                                rule={activeRule}
                                checked={false}
                                onCheck={() => { }}
                                onDelete={() => { }}
                                isRemoving={false}
                                hideDeleteButton
                            />
                        </div>
                    ) : activeGroupSection ? (
                        <div className="flex items-center gap-2 w-full px-1 py-1.5 rounded-md bg-background ring-1 ring-[var(--tailwind-colors-rdns-600)] shadow-lg shadow-black/30 cursor-grabbing">
                            <GripVertical className="w-4 h-4 text-[var(--tailwind-colors-slate-500)] shrink-0" />
                            <Folder className="w-5 h-5 md:w-4 md:h-4 shrink-0" />
                            <span className="font-medium text-base md:text-sm truncate">{activeGroupSection.name}</span>
                            <span className="text-sm md:text-xs text-[var(--tailwind-colors-slate-400)] shrink-0">{activeGroupSection.items.length}</span>
                        </div>
                    ) : null}
                </DragOverlay>
            </DndContext>

            <Dialog open={groupToDelete !== null} onOpenChange={(open) => { if (!open) setGroupToDelete(null); }}>
                <DialogContent className="sm:max-w-md">
                    <DialogHeader>
                        <DialogTitle>Delete group</DialogTitle>
                        <DialogDescription className="break-words">
                            Delete the group <span className="font-medium text-foreground break-all">“{groupToDelete}”</span>? Its rules move to Ungrouped — they are not deleted.
                        </DialogDescription>
                    </DialogHeader>
                    <DialogFooter>
                        <Button
                            variant="cancel"
                            size="lg"
                            className="flex-1 min-w-32 font-medium"
                            onClick={() => setGroupToDelete(null)}
                            disabled={loading}
                        >
                            Cancel
                        </Button>
                        <Button
                            variant="default"
                            size="lg"
                            className="flex-1 min-w-32 bg-[var(--tailwind-colors-red-600)] text-white hover:bg-[var(--tailwind-colors-red-400)]"
                            onClick={confirmDeleteGroup}
                            disabled={loading}
                        >
                            Delete group
                        </Button>
                    </DialogFooter>
                </DialogContent>
            </Dialog>
        </div>
    );
}

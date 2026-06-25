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
import { Tooltip } from "@/components/ui/tooltip";
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

const UNGROUPED = "";
const CONTAINER_PREFIX = "group:";
const NEW_GROUP_ZONE = "group:__new__";

const containerId = (group: string) => `${CONTAINER_PREFIX}${group}`;
const isContainerId = (id: string) => id.startsWith(CONTAINER_PREFIX);
const groupFromContainer = (id: string) => id.slice(CONTAINER_PREFIX.length);

export interface CustomRulesCardProps {
    rules: ModelCustomRule[];
    groupNotes: Record<string, string>;
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
}): JSX.Element {
    const [editingNote, setEditingNote] = useState(false);
    const [noteDraft, setNoteDraft] = useState(note);
    const [renaming, setRenaming] = useState(false);
    const [nameDraft, setNameDraft] = useState(name);

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
        <div className="flex items-center gap-1 w-full px-1 py-1.5 mt-2">
            <button
                type="button"
                onClick={onToggle}
                className="flex items-center gap-3 md:gap-2 min-w-0 py-1.5 md:py-0 text-[var(--tailwind-colors-slate-100)] shrink-0 cursor-pointer"
            >
                {collapsed ? <Folder className="w-5 h-5 md:w-4 md:h-4" /> : <FolderOpen className="w-5 h-5 md:w-4 md:h-4" />}
                <span className="font-medium text-base md:text-sm truncate">{name}</span>
                <span className="text-sm md:text-xs text-[var(--tailwind-colors-slate-400)]">{count}</span>
                {note && !editingNote && (
                    <Tooltip content={note}>
                        <span className="inline-flex items-center text-[var(--tailwind-colors-slate-400)]" aria-label="Group note">
                            <StickyNote className="w-5 h-5 md:w-4 md:h-4" />
                        </span>
                    </Tooltip>
                )}
            </button>

            {editingNote ? (
                <div className="flex items-center gap-1 min-w-0">
                    <Input
                        value={noteDraft}
                        onChange={(e) => setNoteDraft(e.target.value.slice(0, 280))}
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
                    <DropdownMenuContent align="end">
                        <DropdownMenuItem onClick={() => setRenaming(true)}>
                            <Pencil className="w-4 h-4 mr-2" /> Rename group
                        </DropdownMenuItem>
                        <DropdownMenuItem onClick={() => setEditingNote(true)}>
                            <StickyNote className="w-4 h-4 mr-2" /> {note ? "Edit comment" : "Add comment"}
                        </DropdownMenuItem>
                        <DropdownMenuItem
                            className="text-[var(--tailwind-colors-rdns-600)] focus:text-[var(--tailwind-colors-rdns-600)]"
                            onClick={onDelete}
                        >
                            <Trash2 className="w-4 h-4 mr-2" /> Delete group
                        </DropdownMenuItem>
                    </DropdownMenuContent>
                </DropdownMenu>
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
                "flex items-center justify-center gap-2 w-full min-h-16 mt-1 rounded-md border border-dashed text-xs font-medium transition-colors cursor-pointer",
                isOver
                    ? "border-[var(--tailwind-colors-rdns-600)] bg-[var(--tailwind-colors-rdns-600)]/5 text-[var(--tailwind-colors-rdns-600)]"
                    : "border-[var(--tailwind-colors-slate-light-400)] text-[var(--tailwind-colors-slate-light-600)] hover:border-[var(--tailwind-colors-slate-light-500)] hover:text-[var(--tailwind-colors-slate-light-800)] dark:border-[var(--tailwind-colors-slate-700)] dark:text-[var(--tailwind-colors-slate-400)] dark:hover:text-[var(--tailwind-colors-slate-200)] dark:hover:border-[var(--tailwind-colors-slate-500)]",
            ].join(" ")}
        >
            <FolderPlus className="w-4 h-4" />
            {isOver ? "Drop to create a new group" : "New group"}
        </button>
    );
}

export default function CustomRulesCard({
    rules,
    groupNotes,
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
    allSelected,
    selectedCount,
    handleBulkDelete,
    loading,
    type,
    searchQuery,
}: CustomRulesCardProps): JSX.Element {
    const [removingIds, setRemovingIds] = useState<string[]>([]);
    const [collapsed, setCollapsed] = useState<Set<string>>(new Set());
    const [activeId, setActiveId] = useState<string | null>(null);
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

    // Group existence = union of the registry (groupNotes) and any rule's label.
    const groupNames = useMemo(() => {
        const set = new Set<string>();
        for (const r of localRules) { const g = r.group ?? ""; if (g !== "") set.add(g); }
        for (const k of Object.keys(groupNotes)) { if (k !== "") set.add(k); }
        return Array.from(set).sort((a, b) => a.localeCompare(b, undefined, { sensitivity: "base" }));
    }, [localRules, groupNotes]);

    // Sections: Ungrouped first, then named groups alphabetically. Empty groups included.
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
        setActiveId(String(event.active.id));
    }, []);

    // Live cross-section move: when the row hovers a different group, relabel it and
    // splice it into that group so the UI reflects the move mid-drag.
    const handleDragOver = useCallback((event: DragOverEvent) => {
        const { active, over } = event;
        if (!over) return;
        const activeKey = String(active.id);
        const overKey = String(over.id);
        if (overKey === NEW_GROUP_ZONE) return; // resolved on drop

        const targetGroup = isContainerId(overKey) ? groupFromContainer(overKey) : groupOf(overKey);
        const activeGroup = groupOf(activeKey);
        if (targetGroup === activeGroup) return;

        setLocalRules(prev => {
            const activeIdx = prev.findIndex(r => r.id === activeKey);
            if (activeIdx < 0) return prev;
            const next = [...prev];
            const [moved] = next.splice(activeIdx, 1);
            const relabelled = { ...moved, group: targetGroup === UNGROUPED ? "" : targetGroup };

            let insertIdx: number;
            if (isContainerId(overKey)) {
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
        if (!isContainerId(overKey) && activeKey !== overKey) {
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
            void onMoveRule(orderedIds, activeKey, finalGroup);
        } else {
            // Only persist if order actually changed.
            const prevIds = sortedFromProps.map(r => r.id);
            if (orderedIds.join("|") !== prevIds.join("|")) void onReorder(orderedIds);
        }
    }, [localRules, rules, sortedFromProps, onMoveRule, onReorder]);

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

    const activeRule = activeId ? localRules.find(r => r.id === activeId) : null;
    const hasNamedGroups = groupNames.length > 0;

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
                    {sections.map((section) => {
                        const isCollapsed = collapsed.has(section.name);
                        const sectionIds = section.items.map(r => r.id);
                        const isEmpty = section.items.length === 0;
                        // Hide the empty Ungrouped section unless named groups exist (so it can
                        // serve as a "remove from group" target).
                        if (section.name === UNGROUPED && isEmpty && !hasNamedGroups) return null;

                        return (
                            <div key={section.name || "__ungrouped__"} className="w-full">
                                {section.name !== UNGROUPED ? (
                                    <GroupHeader
                                        name={section.name}
                                        count={section.items.length}
                                        collapsed={isCollapsed}
                                        onToggle={() => setCollapsed(prev => {
                                            const next = new Set(prev);
                                            if (next.has(section.name)) next.delete(section.name);
                                            else next.add(section.name);
                                            return next;
                                        })}
                                        note={groupNotes[section.name] ?? ""}
                                        onSaveNote={(n) => onSaveGroupNote(section.name, n)}
                                        onRename={(to) => onRenameGroup(section.name, to)}
                                        onDelete={() => setGroupToDelete(section.name)}
                                        loading={loading}
                                    />
                                ) : (
                                    hasNamedGroups && (
                                        <div className="px-1 py-1.5 mt-2 text-xs font-medium text-[var(--tailwind-colors-slate-400)] uppercase tracking-wide">
                                            Ungrouped
                                        </div>
                                    )
                                )}
                                {/* grid-template-rows 0fr<->1fr animates the section height
                                    fluidly without measuring; the inner wrapper clips during
                                    the transition. Kept mounted so it stays a drop target. */}
                                <div
                                    className={`grid transition-[grid-template-rows] duration-300 ease-in-out motion-reduce:transition-none ${isCollapsed ? "grid-rows-[0fr]" : "grid-rows-[1fr]"}`}
                                >
                                    <div className={`min-h-0 overflow-hidden transition-opacity duration-200 ${isCollapsed ? "opacity-0" : "opacity-100"}`}>
                                        <SortableContext items={sectionIds} strategy={verticalListSortingStrategy}>
                                            <DroppableSection group={section.name} isEmpty={isEmpty}>
                                                {section.items.map((rule) => (
                                                    <SortableEntry
                                                        key={rule.id}
                                                        rule={rule}
                                                        checked={selectedIds.includes(rule.id)}
                                                        onCheck={onCheck}
                                                        onDelete={handleEntryDelete}
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
                            </div>
                        );
                    })}

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
                                onCheck={() => {}}
                                onDelete={() => {}}
                                isRemoving={false}
                                hideDeleteButton
                            />
                        </div>
                    ) : null}
                </DragOverlay>
            </DndContext>

            <Dialog open={groupToDelete !== null} onOpenChange={(open) => { if (!open) setGroupToDelete(null); }}>
                <DialogContent className="sm:max-w-md">
                    <DialogHeader>
                        <DialogTitle>Delete group</DialogTitle>
                        <DialogDescription>
                            Delete the group <span className="font-medium text-foreground">“{groupToDelete}”</span>? Its rules move to Ungrouped — they are not deleted.
                        </DialogDescription>
                    </DialogHeader>
                    <DialogFooter>
                        <Button variant="ghost" onClick={() => setGroupToDelete(null)} disabled={loading}>
                            Cancel
                        </Button>
                        <Button
                            className="bg-[var(--tailwind-colors-rdns-600)] text-[var(--tailwind-colors-slate-900)]"
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

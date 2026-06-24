import { useCallback, useEffect, useMemo, useState, type JSX } from "react";
import {
    DndContext,
    closestCenter,
    KeyboardSensor,
    PointerSensor,
    useSensor,
    useSensors,
    type DragEndEvent,
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
    Check,
    ChevronDown,
    ChevronRight,
    GripVertical,
    MinusIcon,
    Pencil,
    StickyNote,
    Trash2,
    X,
} from "lucide-react";
import type { ModelCustomRule } from "@/api/client/api";
import NoRulesExist from "@/pages/custom_rules/NoRulesExist";
import CustomRuleEntry from "@/pages/custom_rules/Entry";

export interface CustomRulesCardProps {
    rules: ModelCustomRule[];
    groupNotes: Record<string, string>;
    selectedIds: string[];
    onCheck: (id: string, checked: boolean) => void;
    onDelete: (id: string) => void | Promise<void>;
    onEdit: (rule: ModelCustomRule) => void;
    onReorder: (orderedIds: string[]) => void | Promise<void>;
    onSaveGroupNote: (group: string, note: string | null) => void | Promise<void>;
    allSelected: boolean;
    selectedCount: number;
    handleBulkDelete: () => void | Promise<void>;
    loading: boolean;
    type: "denied" | "allowed";
    searchQuery: string;
}

const UNGROUPED = "";

// SortableEntry wraps a rule row with dnd-kit's sortable mechanics, exposing a
// dedicated grip handle so the row's checkbox / edit / delete keep their clicks.
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
        opacity: isDragging ? 0.5 : 1,
        zIndex: isDragging ? 10 : undefined,
        position: "relative",
    };

    const handle = draggable ? (
        <button
            type="button"
            className="flex items-center justify-center cursor-grab touch-none text-[var(--tailwind-colors-slate-400)] hover:text-[var(--tailwind-colors-slate-200)] shrink-0"
            aria-label="Drag to reorder"
            {...attributes}
            {...listeners}
        >
            <GripVertical className="w-4 h-4" />
        </button>
    ) : undefined;

    return (
        <div ref={setNodeRef} style={style} className="w-full">
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

// GroupHeader renders a collapsible section header with an inline-editable note.
function GroupHeader({
    name,
    count,
    collapsed,
    onToggle,
    note,
    onSaveNote,
    loading,
}: {
    name: string;
    count: number;
    collapsed: boolean;
    onToggle: () => void;
    note: string;
    onSaveNote: (note: string | null) => void;
    loading: boolean;
}): JSX.Element {
    const [editing, setEditing] = useState(false);
    const [draft, setDraft] = useState(note);

    useEffect(() => { setDraft(note); }, [note]);

    const commit = () => { onSaveNote(draft.trim() === "" ? null : draft.trim()); setEditing(false); };
    const cancel = () => { setDraft(note); setEditing(false); };

    return (
        <div className="flex items-center gap-1 w-full px-1 py-1.5 mt-2">
            <button
                type="button"
                onClick={onToggle}
                className="flex items-center gap-2 min-w-0 text-[var(--tailwind-colors-slate-100)] shrink-0"
            >
                {collapsed ? <ChevronRight className="w-4 h-4" /> : <ChevronDown className="w-4 h-4" />}
                <span className="font-medium text-sm truncate">{name}</span>
                <span className="text-xs text-[var(--tailwind-colors-slate-400)]">{count}</span>
                {note && !editing && (
                    <Tooltip content={note}>
                        <span className="inline-flex items-center text-[var(--tailwind-colors-slate-400)]" aria-label="Group note">
                            <StickyNote className="w-4 h-4" />
                        </span>
                    </Tooltip>
                )}
            </button>

            {editing ? (
                <div className="flex items-center gap-1 min-w-0">
                    <Input
                        value={draft}
                        onChange={(e) => setDraft(e.target.value.slice(0, 280))}
                        onKeyDown={(e) => {
                            if (e.key === "Enter") { e.preventDefault(); commit(); }
                            else if (e.key === "Escape") { e.preventDefault(); cancel(); }
                        }}
                        placeholder="Group note"
                        className="h-7 w-48 max-w-full"
                        autoFocus
                    />
                    <Button
                        variant="ghost"
                        size="sm"
                        className="h-7 w-7 p-0 shrink-0"
                        disabled={loading}
                        aria-label="Save group note"
                        onClick={commit}
                    >
                        <Check className="w-4 h-4 text-[var(--tailwind-colors-rdns-600)]" />
                    </Button>
                    <Button
                        variant="ghost"
                        size="sm"
                        className="h-7 w-7 p-0 shrink-0"
                        aria-label="Cancel"
                        onClick={cancel}
                    >
                        <X className="w-4 h-4 text-[var(--tailwind-colors-slate-400)]" />
                    </Button>
                </div>
            ) : (
                <Button
                    variant="ghost"
                    size="sm"
                    className="h-7 w-7 p-0 shrink-0"
                    aria-label="Edit group note"
                    onClick={() => setEditing(true)}
                >
                    <Pencil className="w-3.5 h-3.5 text-[var(--tailwind-colors-slate-400)]" />
                </Button>
            )}
        </div>
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
    onSaveGroupNote,
    allSelected,
    selectedCount,
    handleBulkDelete,
    loading,
    type,
    searchQuery,
}: CustomRulesCardProps): JSX.Element {
    const [removingIds, setRemovingIds] = useState<string[]>([]);
    const [collapsed, setCollapsed] = useState<Set<string>>(new Set());

    // localRules mirrors the prop list (sorted by display order) so a drag can be
    // reflected optimistically before the persisted refetch lands.
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

    // Build display sections: ungrouped first, then named groups in first-appearance order.
    const sections = useMemo(() => {
        const order: string[] = [];
        const byGroup = new Map<string, ModelCustomRule[]>();
        for (const r of localRules) {
            const g = r.group ?? UNGROUPED;
            if (!byGroup.has(g)) { byGroup.set(g, []); order.push(g); }
            byGroup.get(g)!.push(r);
        }
        // Keep ungrouped at the top if present.
        order.sort((a, b) => (a === UNGROUPED ? -1 : b === UNGROUPED ? 1 : 0));
        return order.map(name => ({ name, items: byGroup.get(name)! }));
    }, [localRules]);

    const groupOf = useCallback(
        (id: string) => localRules.find(r => r.id === id)?.group ?? UNGROUPED,
        [localRules],
    );

    // Drag is allowed only while not searching (search filters the list, so order
    // would be ambiguous) and when nothing is bulk-selected.
    const draggable = searchQuery.trim().length === 0 && !allSelected && !loading;

    const handleDragEnd = useCallback((event: DragEndEvent) => {
        const { active, over } = event;
        if (!over || active.id === over.id) return;
        // Only reorder within the same group; changing groups is done via Edit.
        if (groupOf(String(active.id)) !== groupOf(String(over.id))) return;

        const oldIndex = localRules.findIndex(r => r.id === active.id);
        const newIndex = localRules.findIndex(r => r.id === over.id);
        if (oldIndex < 0 || newIndex < 0) return;

        const next = arrayMove(localRules, oldIndex, newIndex);
        setLocalRules(next);
        void onReorder(next.map(r => r.id));
    }, [groupOf, localRules, onReorder]);

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

            <DndContext sensors={sensors} collisionDetection={closestCenter} onDragEnd={handleDragEnd}>
                <div className="flex flex-col items-start gap-2 w-full">
                    {sections.map((section) => {
                        const isCollapsed = collapsed.has(section.name);
                        const sectionIds = section.items.map(r => r.id);
                        return (
                            <div key={section.name || "__ungrouped__"} className="w-full">
                                {section.name !== UNGROUPED && (
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
                                        loading={loading}
                                    />
                                )}
                                {!isCollapsed && (
                                    <SortableContext items={sectionIds} strategy={verticalListSortingStrategy}>
                                        <div className="flex flex-col items-start gap-2 w-full">
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
                                        </div>
                                    </SortableContext>
                                )}
                            </div>
                        );
                    })}
                </div>
            </DndContext>
        </div>
    );
}

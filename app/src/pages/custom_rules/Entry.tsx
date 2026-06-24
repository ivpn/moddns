import React, { useState, useEffect, type ReactNode } from "react";
import { Card, CardContent } from "@/components/ui/card";
import { Checkbox } from "@/components/ui/checkbox";
import { Button } from "@/components/ui/button";
import { Tooltip } from "@/components/ui/tooltip";
import { Pencil, StickyNote, Trash2 } from "lucide-react";
import type { ModelCustomRule } from "@/api/client/api";

interface CustomRuleEntryProps {
    rule: ModelCustomRule;
    checked: boolean;
    onCheck: (id: string, checked: boolean) => void;
    onDelete: (id: string) => void;
    onEdit?: (rule: ModelCustomRule) => void;
    isRemoving: boolean;
    hideDeleteButton?: boolean;
    // dragHandle, when provided, is rendered at the start of the row (a grip the
    // user drags to reorder). The sortable wrapper owns the drag mechanics.
    dragHandle?: ReactNode;
}

const CustomRuleEntry: React.FC<CustomRuleEntryProps> = ({
    rule,
    checked,
    onCheck,
    onDelete,
    onEdit,
    isRemoving,
    hideDeleteButton = false,
    dragHandle,
}) => {
    const [isVisible, setIsVisible] = useState(false);

    useEffect(() => {
        const timeout = setTimeout(() => setIsVisible(true), 10);
        return () => clearTimeout(timeout);
    }, []);


    const domain = rule.value?.replace(/\.$/, "") ?? "";
    const note = rule.note?.trim() ?? "";

    return (
        <Card
            className={`w-full h-10 bg-transparent dark:bg-[var(--variable-collection-surface)] border border-[var(--tailwind-colors-slate-light-300)] dark:border-transparent transition-opacity duration-300 ${isVisible && !isRemoving ? "opacity-100" : "opacity-0"}`}
        >
            <CardContent className="flex items-center justify-between relative self-stretch w-full h-full p-0 px-3">
                <div className="flex items-center gap-4 relative flex-1 min-w-0">
                    {dragHandle}
                    <Checkbox
                        checked={checked}
                        onCheckedChange={val => onCheck(rule.id, Boolean(val))}
                        className="w-4 h-4 border-solid border-[var(--tailwind-colors-rdns-600)]"
                    />
                    <div className="inline-flex items-center gap-2 relative min-w-0">
                        <div className="relative w-fit font-text-sm-leading-5-normal font-normal text-foreground text-sm tracking-normal leading-5 whitespace-nowrap overflow-hidden text-ellipsis">
                            {domain}
                        </div>
                        {note && (
                            <Tooltip content={note}>
                                <span
                                    className="inline-flex items-center text-[var(--tailwind-colors-slate-400)] shrink-0"
                                    aria-label="Rule note"
                                >
                                    <StickyNote className="w-4 h-4" />
                                </span>
                            </Tooltip>
                        )}
                    </div>
                </div>

                <div className="inline-flex items-center gap-1 relative flex-[0_0_auto]">
                    {!hideDeleteButton && onEdit && (
                        <Button
                            variant="ghost"
                            size="sm"
                            className="flex w-10 h-10 items-center justify-center rounded-[var(--primitives-radius-radius-md)] hover:!bg-[var(--tailwind-colors-rdns-600)] group"
                            onClick={() => onEdit(rule)}
                            disabled={isRemoving}
                            aria-label="Edit rule"
                        >
                            <Pencil className="w-4 h-4 text-[var(--tailwind-colors-rdns-600)] group-hover:text-[var(--tailwind-colors-slate-900)] transition-colors" />
                        </Button>
                    )}

                    {!hideDeleteButton && (
                        <Button
                            variant="ghost"
                            size="sm"
                            className="flex w-10 h-10 items-center justify-center rounded-[var(--primitives-radius-radius-md)] hover:!bg-[var(--tailwind-colors-rdns-600)] group"
                            onClick={() => onDelete(rule.id)}
                            disabled={isRemoving}
                            aria-label="Delete rule"
                        >
                            <Trash2 className="w-4 h-4 text-[var(--tailwind-colors-rdns-600)] group-hover:text-[var(--tailwind-colors-slate-900)] transition-colors" />
                        </Button>
                    )}
                </div>
            </CardContent>
        </Card>
    );
};

export default CustomRuleEntry;

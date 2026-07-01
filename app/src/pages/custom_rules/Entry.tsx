import React, { useState, useEffect, type ReactNode } from "react";
import { Card, CardContent } from "@/components/ui/card";
import { Checkbox } from "@/components/ui/checkbox";
import { Button } from "@/components/ui/button";
import { Pencil, Trash2 } from "lucide-react";
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
                    {/* Domain + always-visible note. The note is real text in reading order
                        (no hover/tooltip), so it works on touch, keyboard, and screen readers;
                        it clamps to 2 lines (notes are capped at 80 chars, so this rarely clips). */}
                    <div className="flex flex-col min-w-0 gap-0.5">
                        <div className="font-text-sm-leading-5-normal font-normal text-foreground text-sm tracking-normal leading-5 truncate">
                            {domain}
                        </div>
                        {note && (
                            <div className="text-xs leading-4 truncate text-[var(--tailwind-colors-slate-light-600)] dark:text-[var(--tailwind-colors-slate-400)]">
                                {note}
                            </div>
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

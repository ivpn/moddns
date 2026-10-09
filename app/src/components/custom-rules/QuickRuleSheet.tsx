import { useCallback, useEffect, useMemo, useState, type KeyboardEvent, type ReactNode, type RefObject } from "react";
import { ShieldBan, ShieldCheck } from "lucide-react";
import { Sheet, SheetContent, SheetFooter, SheetHeader, SheetTitle } from "@/components/ui/sheet";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group";
import { normalizeRuleValue } from "@/pages/custom_rules/utils";
import { toast } from "sonner";
import { useAppStore } from "@/store/general";
import api from "@/api/api";
import { useScreenDetector } from "@/hooks/useScreenDetector";
import { formatApiError } from "@/lib/apiError";

export type QuickRuleAction = "denylist" | "allowlist";

const ACTION_TO_API: Record<QuickRuleAction, "block" | "allow"> = {
    denylist: "block",
    allowlist: "allow",
};

const ACTION_LABEL: Record<QuickRuleAction, string> = {
    denylist: "Denylist",
    allowlist: "Allowlist",
};

const ACTION_HELPER: Record<QuickRuleAction, string> = {
    denylist: "Blocks the domain and its matching subdomains for the selected profile.",
    allowlist: "Overrides blocklists and always allows the domain when resolving.",
};

const ACTION_SEQUENCE: QuickRuleAction[] = ["denylist", "allowlist"];

interface QuickRuleSheetProps {
    open: boolean;
    onOpenChange: (next: boolean) => void;
    domain?: string;
    defaultAction?: QuickRuleAction;
    /** Called after a rule was created, before the sheet closes. */
    onCreated?: (rule: { value: string; action: QuickRuleAction }) => void;
    /** Adds a description and an action button to the success toast. */
    successToast?: { description: string; action: { label: string; onClick: () => void } };
    /** Replaces the inline error for a value that already has a rule. */
    duplicateNotice?: (value: string) => ReactNode;
    /** Where focus goes when the sheet closes; defaults to the dialog's own trigger logic. */
    returnFocusTo?: RefObject<HTMLElement | null>;
}

const iconClasses = "h-4 w-4";

function prefillDomain(raw: string, mode: "include" | "exact"): string {
    if (mode === "exact") return raw;
    const stripped = raw.replace(/^www\./, "");
    return `*.${stripped}`;
}

const QuickRuleSheet = ({ open, onOpenChange, domain, defaultAction, onCreated, successToast, duplicateNotice, returnFocusTo }: QuickRuleSheetProps) => {
    const [action, setAction] = useState<QuickRuleAction>("denylist");
    const [domainValue, setDomainValue] = useState(domain ?? "");
    const [isSubmitting, setIsSubmitting] = useState(false);
    const [inputError, setInputError] = useState<string | null>(null);
    const [duplicateValue, setDuplicateValue] = useState<string | null>(null);
    const { isDesktop } = useScreenDetector();

    const activeProfile = useAppStore((state) => state.activeProfile);
    const setActiveProfile = useAppStore((state) => state.setActiveProfile);
    const profileDisplayName = activeProfile?.name ?? "current profile";

    useEffect(() => {
        if (!open) return;
        setAction(defaultAction ?? "denylist");
        const subdomainsRule = activeProfile?.settings?.privacy?.custom_rules_subdomains_rule ?? "include";
        setDomainValue(domain ? prefillDomain(domain, subdomainsRule) : "");
        setInputError(null);
        setDuplicateValue(null);
    }, [activeProfile, defaultAction, domain, open]);

    const disabled = useMemo(() => !domainValue.trim() || isSubmitting, [domainValue, isSubmitting]);

    const handleSubmit = async () => {
        const normalized = normalizeRuleValue(domainValue || "");
        if (!normalized) {
            setInputError("Enter a valid domain or hostname.");
            return;
        }
        if (!activeProfile?.profile_id) {
            toast.error("Select a profile before adding custom rules.");
            return;
        }

        setIsSubmitting(true);
        setInputError(null);
        setDuplicateValue(null);
        try {
            const response = await api.Client.profilesApi.apiV1ProfilesIdCustomRulesBatchPost(
                activeProfile.profile_id,
                {
                    action: ACTION_TO_API[action],
                    values: [normalized],
                }
            );

            const createdCount = response.data?.created?.length ?? 0;
            const skipped = response.data?.skipped ?? [];

            if (createdCount > 0) {
                const updated = await api.Client.profilesApi.apiV1ProfilesIdGet(activeProfile.profile_id);
                setActiveProfile(updated.data);
                toast.success(
                    `${normalized} added to the ${ACTION_LABEL[action]}.`,
                    successToast ? { description: successToast.description, action: successToast.action } : undefined,
                );
                onCreated?.({ value: normalized, action });
                onOpenChange(false);
                return;
            }

            if (skipped.length > 0) {
                const first = skipped[0];
                if (duplicateNotice && first?.reason === "duplicate_existing") {
                    setDuplicateValue(normalized);
                    return;
                }
                setInputError(first?.message ?? "Unable to add this entry.");
                toast.warning("Review the highlighted entry before trying again.");
                return;
            }

            toast.info("No changes were applied.");
        } catch (error) {
            console.error(error);
            toast.error(formatApiError(error, "Failed to add custom rule."));
        } finally {
            setIsSubmitting(false);
        }
    };

    const handleDomainKeyDown = (event: KeyboardEvent<HTMLInputElement>) => {
        if (event.key !== "Enter" || event.shiftKey || event.isComposing) {
            return;
        }
        if (!isDesktop) {
            return;
        }
        event.preventDefault();
        if (!disabled) {
            void handleSubmit();
        }
    };

    const handleActionKeyDown = useCallback(
        (event: KeyboardEvent<HTMLDivElement>) => {
            if (event.defaultPrevented) return;
            const key = event.key.toLowerCase();
            if (key === "b" || key === "d") {
                event.preventDefault();
                setAction("denylist");
                return;
            }
            if (key === "a") {
                event.preventDefault();
                setAction("allowlist");
                return;
            }
            if (event.key === "ArrowLeft" || event.key === "ArrowRight") {
                event.preventDefault();
                const delta = event.key === "ArrowRight" ? 1 : -1;
                const currentIndex = ACTION_SEQUENCE.indexOf(action);
                const nextIndex = (currentIndex + delta + ACTION_SEQUENCE.length) % ACTION_SEQUENCE.length;
                setAction(ACTION_SEQUENCE[nextIndex]);
            }
        },
        [action]
    );

    return (
        <Sheet open={open} onOpenChange={onOpenChange}>
            <SheetContent
                side="right"
                className="max-w-[480px] w-full border-l border-[var(--tailwind-colors-slate-800)] bg-[var(--variable-collection-surface)] p-0 gap-0"
                onCloseAutoFocus={event => {
                    const target = returnFocusTo?.current;
                    if (target) {
                        event.preventDefault();
                        target.focus();
                    }
                }}
            >
                <SheetHeader className="gap-1 px-6 pt-6 pb-2 text-left">
                    <SheetTitle>Add custom rule</SheetTitle>
                </SheetHeader>

                <div className="flex flex-col gap-6 px-6 py-6">
                    <div className="space-y-3">
                        <Label className="text-sm font-medium text-[var(--tailwind-colors-slate-200)]">
                            Action
                        </Label>
                        <ToggleGroup
                            type="single"
                            value={action}
                            onValueChange={(value) => value && setAction(value as QuickRuleAction)}
                            className="w-full"
                            variant="outline"
                            onKeyDown={handleActionKeyDown}
                        >
                            <ToggleGroupItem
                                value="denylist"
                                aria-label="Block domain"
                                className="flex-1 px-3 py-3 transition-all duration-500 cursor-pointer data-[state=on]:bg-[var(--tailwind-colors-rdns-600)] data-[state=on]:text-[var(--tailwind-colors-slate-900)] data-[state=on]:border-[var(--tailwind-colors-rdns-600)] [:root:not(.dark)_&]:data-[state=off]:border-[var(--tailwind-colors-slate-light-300)]"
                            >
                                <ShieldBan className={iconClasses} />
                                <span className="text-sm font-medium">Block</span>
                            </ToggleGroupItem>
                            <ToggleGroupItem
                                value="allowlist"
                                aria-label="Allow domain"
                                className="flex-1 px-3 py-3 transition-all duration-500 cursor-pointer data-[state=on]:bg-[var(--tailwind-colors-rdns-600)] data-[state=on]:text-[var(--tailwind-colors-slate-900)] data-[state=on]:border-[var(--tailwind-colors-rdns-600)] [:root:not(.dark)_&]:data-[state=off]:border-[var(--tailwind-colors-slate-light-300)]"
                            >
                                <ShieldCheck className={iconClasses} />
                                <span className="text-sm font-medium">Allow</span>
                            </ToggleGroupItem>
                        </ToggleGroup>
                        <p className="text-xs text-[var(--tailwind-colors-slate-400)]">
                            {ACTION_HELPER[action]}
                        </p>
                    </div>

                    <div className="space-y-2">
                        <Label htmlFor="quick-rule-domain" className="text-sm font-medium text-[var(--tailwind-colors-slate-200)]">
                            Domain
                        </Label>
                        <Input
                            id="quick-rule-domain"
                            placeholder="example.com"
                            value={domainValue}
                            readOnly
                            onKeyDown={handleDomainKeyDown}
                            autoComplete="off"
                            autoCapitalize="none"
                            spellCheck={false}
                            autoCorrect="off"
                            className="caret-transparent cursor-default focus-visible:ring-0"
                        />
                        <p className="text-xs text-[var(--tailwind-colors-slate-500)]">
                            Applies to {profileDisplayName}.
                        </p>
                        {duplicateNotice && duplicateValue && (
                            <p className="text-xs text-[var(--tailwind-colors-rose-400)]" role="alert">
                                {duplicateNotice(duplicateValue)}
                            </p>
                        )}
                        {inputError && (
                            <p className="text-xs text-[var(--tailwind-colors-rose-400)]" role="alert">
                                {inputError}
                            </p>
                        )}
                    </div>
                </div>

                <SheetFooter className="mt-0 flex-col-reverse gap-2 p-0 px-6 pb-6">
                    <Button
                        variant="cancel"
                        onClick={() => onOpenChange(false)}
                        type="button"
                        className="w-full"
                    >
                        Cancel
                    </Button>
                    <Button
                        onClick={handleSubmit}
                        disabled={disabled}
                        className="w-full bg-[var(--tailwind-colors-rdns-600)] text-[var(--tailwind-colors-slate-900)]"
                        type="button"
                    >
                        {isSubmitting ? "Saving..." : `Add to ${ACTION_LABEL[action]}`}
                    </Button>
                </SheetFooter>
            </SheetContent>
        </Sheet>
    );
};

export default QuickRuleSheet;

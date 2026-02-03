import { type ComponentProps, useMemo, useState, type KeyboardEvent } from "react";
import CreatableSelect from "react-select/creatable";
import {
    components,
    type ActionMeta,
    type ControlProps,
    type CSSObjectWithLabel,
    type GroupBase,
    type InputActionMeta,
    type MultiValue,
    type MultiValueGenericProps,
    type MultiValueProps,
    type OptionProps,
    type StylesConfig,
    type Theme,
    type ThemeConfig,
} from "react-select";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import { Plus } from "lucide-react";
import { MAX_RULES_PER_BATCH, normalizeRuleValue, splitRulesFromInput } from "@/pages/custom_rules/utils";

export interface RuleOptionMeta {
    error?: string;
    reason?: string;
}

export interface RuleOption {
    label: string;
    value: string;
    meta?: RuleOptionMeta;
}

interface RuleComposerProps {
    tokens: RuleOption[];
    onTokensChange: (next: RuleOption[]) => void;
    onSubmit: () => void;
    loading: boolean;
    action: "denylist" | "allowlist";
    className?: string;
}

type RuleOptionGroup = GroupBase<RuleOption>;

const selectStyles: StylesConfig<RuleOption, true, RuleOptionGroup> = {
    container: (base: CSSObjectWithLabel) => ({
        ...base,
        width: "100%",
        flex: "1 1 0%",
        minWidth: 0,
        maxWidth: "100%",
    }),
    control: (base: CSSObjectWithLabel, state: ControlProps<RuleOption, true, RuleOptionGroup>) => ({
        ...base,
        width: "100%",
        maxWidth: "100%",
        overflow: "visible",
        backgroundColor: "var(--shadcn-ui-app-background)",
        borderColor: state.isFocused ? "var(--tailwind-colors-slate-400)" : "var(--tailwind-colors-slate-600)",
        boxShadow: "none",
        minHeight: "2.75rem",
        height: "2.75rem",
        paddingLeft: "0.25rem",
        paddingRight: "0.25rem",
        cursor: "text",
        transition: "border-color 0.2s ease",
        ":hover": {
            borderColor: "var(--tailwind-colors-slate-400)",
        },
        "@media (min-width: 1024px)": {
            minHeight: "2.25rem",
            height: "2.25rem",
        },
    }),
    multiValue: (base: CSSObjectWithLabel, state: MultiValueProps<RuleOption, true, RuleOptionGroup>) => ({
        ...base,
        backgroundColor: state.data.meta?.error
            ? "rgba(248, 113, 113, 0.16)"
            : "var(--tailwind-colors-slate-800)",
        border: state.data.meta?.error
            ? "1px solid var(--tailwind-colors-rose-500)"
            : "1px solid transparent",
        borderRadius: "0.375rem",
        paddingRight: "0.25rem",
        flexShrink: 0,
        cursor: "pointer",
    }),
    multiValueLabel: (base: CSSObjectWithLabel, state: MultiValueProps<RuleOption, true, RuleOptionGroup>) => ({
        ...base,
        color: state.data.meta?.error
            ? "var(--tailwind-colors-rose-200)"
            : "var(--tailwind-colors-slate-200)",
        fontSize: "0.8125rem",
        padding: "0.1rem 0.35rem",
        display: "flex",
        alignItems: "center",
        gap: "0.35rem",
    }),
    multiValueRemove: (base: CSSObjectWithLabel, state: MultiValueProps<RuleOption, true, RuleOptionGroup>) => ({
        ...base,
        color: state.data.meta?.error ? "var(--tailwind-colors-rose-200)" : base.color,
        cursor: "pointer",
        ":hover": {
            backgroundColor: state.data.meta?.error
                ? "var(--tailwind-colors-rose-500)"
                : "var(--tailwind-colors-slate-800)",
            color: "var(--tailwind-colors-slate-200)",
        },
    }),
    menuPortal: (base: CSSObjectWithLabel) => ({
        ...base,
        zIndex: 60,
    }),
    menu: (base: CSSObjectWithLabel) => ({
        ...base,
        backgroundColor: "var(--shadcn-ui-app-background)",
        border: "1px solid var(--tailwind-colors-slate-400)",
    }),
    option: (base: CSSObjectWithLabel, state: OptionProps<RuleOption, true, RuleOptionGroup>) => ({
        ...base,
        backgroundColor: state.isFocused
            ? "var(--tailwind-colors-slate-700)"
            : "transparent",
        color: "var(--tailwind-colors-slate-100)",
    }),
    placeholder: (base: CSSObjectWithLabel) => ({
        ...base,
        color: "var(--tailwind-colors-slate-400)",
    }),
    valueContainer: (base: CSSObjectWithLabel) => ({
        ...base,
        flex: "1 1 0%",
        minWidth: 0,
        maxWidth: "100%",
        flexWrap: "nowrap",
        overflowX: "auto",
        overflowY: "hidden",
        gap: "0.35rem",
        paddingRight: "0.25rem",
        WebkitOverflowScrolling: "touch",
        scrollbarWidth: "thin",
    }),
    input: (base: CSSObjectWithLabel) => ({
        ...base,
        color: "var(--tailwind-colors-slate-100)",
        caretColor: "var(--tailwind-colors-rdns-400)",
        minWidth: "6rem",
    }),
};

const selectTheme: ThemeConfig = (theme: Theme) => ({
    ...theme,
    colors: {
        ...theme.colors,
        primary: "var(--tailwind-colors-rdns-600)",
        primary25: "var(--tailwind-colors-slate-700)",
        neutral0: "var(--shadcn-ui-app-background)",
        neutral5: "var(--tailwind-colors-slate-850, #1f2937)",
        neutral10: "var(--tailwind-colors-slate-700)",
        neutral20: "var(--tailwind-colors-slate-600)",
        neutral30: "var(--tailwind-colors-slate-500)",
        neutral40: "var(--tailwind-colors-slate-400)",
        neutral50: "var(--tailwind-colors-slate-400)",
    },
});

const MultiValueLabel = (props: MultiValueGenericProps<RuleOption, true, RuleOptionGroup>) => {
    const { data } = props;

    return (
        <components.MultiValueLabel {...props}>
            <span className="truncate" title={data.value}>
                {data.label}
            </span>
            {data.meta?.error && (
                <span className="text-xs font-medium text-[var(--tailwind-colors-rose-300)]" title={data.meta.error}>
                    !
                </span>
            )}
        </components.MultiValueLabel>
    );
};

const DropdownIndicator = () => null;
const IndicatorSeparator = () => null;
const Menu = () => null;

const CustomInput = (props: ComponentProps<typeof components.Input>) => (
    <components.Input {...props} autoCapitalize="none" spellCheck={false} autoCorrect="off" />
);

export function RuleComposer({
    tokens,
    onTokensChange,
    onSubmit,
    loading,
    action,
    className,
}: RuleComposerProps) {
    const [inputValue, setInputValue] = useState("");

    const existingValues = useMemo(() => new Set(tokens.map((token) => token.value)), [tokens]);

    const menuPortalTarget = typeof document !== "undefined" ? document.body : undefined;

    const hasValidTokens = tokens.some((token) => !token.meta?.error);
    const availableSlots = MAX_RULES_PER_BATCH - tokens.length;

    const addTokens = (rawValues: string[]) => {
        if (rawValues.length === 0) {
            return;
        }

        const normalizedSet = new Set<string>();
        const duplicates: string[] = [];
        const added: RuleOption[] = [];
        const overflow: string[] = [];

        let slots = availableSlots;

        rawValues.forEach((raw) => {
            if (slots <= 0) {
                const normalizedOverflow = normalizeRuleValue(raw);
                if (normalizedOverflow) {
                    overflow.push(normalizedOverflow);
                }
                return;
            }

            const normalized = normalizeRuleValue(raw);
            if (!normalized) {
                return;
            }

            if (existingValues.has(normalized) || normalizedSet.has(normalized)) {
                duplicates.push(normalized);
                return;
            }

            normalizedSet.add(normalized);
            added.push({ label: normalized, value: normalized });
            slots -= 1;
        });

        if (added.length > 0) {
            onTokensChange([...tokens, ...added]);
        }

        if (duplicates.length > 0) {
            toast.warning(`${duplicates.length} entr${duplicates.length === 1 ? "y" : "ies"} skipped as duplicates.`);
        }

        if (overflow.length > 0) {
            toast.error(`Only ${MAX_RULES_PER_BATCH} entries can be submitted at once.`);
        }

        setInputValue("");
    };

    const handleCreateOption = (value: string) => {
        if (value.trim().length === 0) {
            return;
        }
        addTokens(splitRulesFromInput(value));
    };

    const handleInputChange = (value: string, meta: InputActionMeta) => {
        if (meta.action !== "input-change") {
            return inputValue;
        }

        if (/[\s,;]/.test(value)) {
            addTokens(splitRulesFromInput(value));
            return "";
        }

        setInputValue(value);
        return value;
    };

    const handleKeyDown = (event: KeyboardEvent<HTMLDivElement>) => {
        if (["Enter", "Tab", ","].includes(event.key)) {
            if (inputValue.trim()) {
                event.preventDefault();
                addTokens(splitRulesFromInput(inputValue));
                return;
            }

            if (event.key === "Enter" && hasValidTokens && !loading) {
                event.preventDefault();
                onSubmit();
            }
        }
    };

    const handleChange = (
        nextValue: MultiValue<RuleOption>,
        actionMeta: ActionMeta<RuleOption>
    ) => {
        if (actionMeta.action === "clear") {
            onTokensChange([]);
            return;
        }
        onTokensChange(nextValue.map((option: RuleOption) => ({ ...option })));
    };

    return (
        <div className={cn("flex w-full items-start gap-2.5", className)}>
            <div className="flex-1 min-w-0">
                <CreatableSelect<RuleOption, true>
                    instanceId={`rule-composer-${action}`}
                    isMulti
                    closeMenuOnSelect={false}
                    components={{
                        MultiValueLabel,
                        DropdownIndicator,
                        IndicatorSeparator,
                        Menu,
                        Input: CustomInput,
                    }}
                    classNamePrefix="rule-composer"
                    placeholder="Paste or type domains, IPs, or ASNs"
                    value={tokens}
                    inputValue={inputValue}
                    onChange={handleChange}
                    onCreateOption={handleCreateOption}
                    onInputChange={handleInputChange}
                    onKeyDown={handleKeyDown}
                    isDisabled={loading}
                    styles={selectStyles}
                    theme={selectTheme}
                    menuPortalTarget={menuPortalTarget}
                    menuIsOpen={false}
                    hideSelectedOptions
                    captureMenuScroll
                />
            </div>
            <Button
                className="flex items-center justify-center w-11 h-11 md:w-auto md:h-9 md:px-4 md:gap-2 rounded-md bg-[var(--tailwind-colors-rdns-600)] text-[var(--tailwind-colors-slate-900)]"
                onClick={onSubmit}
                disabled={loading || !hasValidTokens}
                aria-label={`Add to ${action === "denylist" ? "Denylist" : "Allowlist"}`}
                type="button"
            >
                <Plus className="w-5 h-5" />
                <span className="hidden md:inline text-sm font-medium leading-6 whitespace-nowrap">
                    Add to <span className="capitalize">{action}</span>
                </span>
            </Button>
        </div>
    );
}

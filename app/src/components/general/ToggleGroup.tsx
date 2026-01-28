import React from "react";
import { ToggleGroup as UIToggleGroup, ToggleGroupItem as UIToggleGroupItem } from "@/components/ui/toggle-group";
import { Check, OctagonX } from "lucide-react";

export interface ToggleOption {
    value: string;
    label: string;
    icon?: "check" | "octagon-x" | React.ReactNode;
    selected?: boolean;
}

export interface ToggleGroupProps {
    options: ToggleOption[];
    value: string;
    onChange: (value: string) => void;
    variant?: "outline" | "default";
    className?: string;
    itemClassName?: string;
    groupProps?: React.ComponentProps<typeof UIToggleGroup>;
    itemProps?: Partial<React.ComponentProps<typeof UIToggleGroupItem>>;
}

const renderIcon = (
    icon?: "check" | "octagon-x" | React.ReactNode,
    selected?: boolean
) => {
    // If not selected, always gray (slate-400)
    // If selected: Check is green (rdns-600), OctagonX is red (red-600)
    if (icon === "octagon-x") {
        return (
            <OctagonX
                className="w-5 h-5"
                color={
                    selected
                        ? "var(--tailwind-colors-red-600)"
                        : "var(--tailwind-colors-slate-400)"
                }
            />
        );
    }
    if (icon === "check") {
        return (
            <Check
                className="w-5 h-5"
                color={
                    selected
                        ? "var(--tailwind-colors-rdns-600)"
                        : "var(--tailwind-colors-slate-400)"
                }
            />
        );
    }
    if (React.isValidElement(icon)) return icon;
    return null;
};

const ToggleGroup: React.FC<ToggleGroupProps> = ({
    options,
    value,
    onChange,
    variant = "outline",
    className = "",
    itemClassName = "",
    groupProps = {},
    itemProps = {},
}) => {
    return (
        <UIToggleGroup
            type="single"
            aria-label="Toggle Group"
            variant={variant}
            className={`
                p-0.5 rounded-[var(--primitives-radius-radius)] gap-0
                transition-colors duration-200 bg-[var(--shadcn-ui-app-muted)]
                ${className}
            `}
            value={value}
            onValueChange={onChange}
            {...groupProps}
        >
            {options.map((option) => {
                const isSelected = value === option.value || option.selected;
                return (
                    <UIToggleGroupItem
                        key={option.value}
                        value={option.value}
                        aria-label={option.label}
                        className={`
                            cursor-pointer
                            flex flex-row items-center justify-center
                            min-w-[72px] h-9 px-3 py-0
                            transition-all duration-200 ease-in-out
                            data-[state=on]:bg-[var(--variable-collection-surface)] data-[state=on]:shadow-[0_2px_8px_0_rgba(18,164,149,0.10)]
                            data-[state=off]:bg-transparent
                            ${itemClassName}
                        `}
                        style={{
                            borderRadius: "var(--tailwind-primitives-border-radius-rounded-lg)",
                        }}
                        {...itemProps}
                    >
                        <span className="flex items-center gap-2">
                            {renderIcon(option.icon, isSelected)}
                            <span
                                className={`
                                    font-medium text-xs text-left leading-5
                                    transition-colors duration-200
                                    ${isSelected
                                        ? "text-[var(--shadcn-ui-app-foreground)]"
                                        : "text-[var(--shadcn-ui-app-muted-foreground)]"
                                    }
                                `}
                                style={{
                                    fontFamily: "'Roboto Flex', Helvetica",
                                }}
                            >
                                {option.label}
                            </span>
                        </span>
                    </UIToggleGroupItem>
                );
            })}
        </UIToggleGroup>
    );
};

export default ToggleGroup;

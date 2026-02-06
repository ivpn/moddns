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
    if (icon === "octagon-x") {
        return (
            <OctagonX
                className="w-3.5 h-3.5 sm:w-4 sm:h-4"
                color={selected ? "white" : "var(--tailwind-colors-slate-400)"}
                strokeWidth={selected ? 2.5 : 2}
            />
        );
    }
    if (icon === "check") {
        return (
            <Check
                className="w-3.5 h-3.5 sm:w-4 sm:h-4"
                color={selected ? "white" : "var(--tailwind-colors-slate-400)"}
                strokeWidth={selected ? 2.5 : 2}
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
    variant = "default",
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
                p-px !rounded-full gap-0
                transition-colors duration-200
                bg-[var(--tailwind-colors-slate-800)]/50
                shadow-[inset_0_1px_3px_rgba(0,0,0,0.3)]
                ring-1 ring-[var(--tailwind-colors-slate-700)]/25

                [:root:not(.dark)_&]:bg-[var(--tailwind-colors-slate-light-200)]/80
                [:root:not(.dark)_&]:shadow-[inset_0_1px_3px_rgba(0,0,0,0.08)]
                [:root:not(.dark)_&]:ring-[var(--tailwind-colors-slate-light-300)]

                !border-none !outline-none
                ${className}
            `}
            value={value}
            onValueChange={onChange}
            {...groupProps}
        >
            {options.map((option) => {
                const isSelected = value === option.value || option.selected;
                const isNegative = option.icon === "octagon-x";
                return (
                    <UIToggleGroupItem
                        key={option.value}
                        value={option.value}
                        aria-label={option.label}
                        className={`
                            cursor-pointer
                            flex flex-row items-center justify-center
                            min-w-[62px] sm:min-w-[74px] h-10 sm:h-9 px-3 sm:px-4 py-0
                            transition-all duration-300 ease-[cubic-bezier(0.34,1.56,0.64,1)]
                            !rounded-full

                            ${isNegative
                                ? `data-[state=on]:bg-[var(--tailwind-colors-red-600)]/90
                                   data-[state=on]:!shadow-[0_1px_6px_-1px_rgba(220,38,38,0.5),0_1px_2px_rgba(0,0,0,0.15)]
                                   [:root:not(.dark)_&]:data-[state=on]:bg-[var(--tailwind-colors-red-600)]/90
                                   [:root:not(.dark)_&]:data-[state=on]:!shadow-[0_1px_8px_-1px_rgba(220,38,38,0.4),0_1px_3px_rgba(0,0,0,0.1)]`
                                : `data-[state=on]:bg-[var(--tailwind-colors-rdns-600)]/90
                                   data-[state=on]:!shadow-[0_1px_6px_-1px_rgba(18,164,149,0.5),0_1px_2px_rgba(0,0,0,0.15)]
                                   [:root:not(.dark)_&]:data-[state=on]:bg-[var(--tailwind-colors-rdns-600)]/90
                                   [:root:not(.dark)_&]:data-[state=on]:!shadow-[0_1px_8px_-1px_rgba(18,164,149,0.4),0_1px_3px_rgba(0,0,0,0.1)]`
                            }
                            data-[state=on]:scale-[1.02]

                            data-[state=off]:bg-transparent
                            data-[state=off]:!shadow-none
                            data-[state=off]:hover:bg-[var(--tailwind-colors-slate-700)]/30

                            [:root:not(.dark)_&]:data-[state=off]:bg-transparent
                            [:root:not(.dark)_&]:data-[state=off]:!shadow-none
                            [:root:not(.dark)_&]:data-[state=off]:hover:bg-[var(--tailwind-colors-slate-light-300)]/60

                            !outline-none !ring-0 !border-none
                            ${itemClassName}
                        `}
                        {...itemProps}
                    >
                        <span className="flex items-center gap-1.5">
                            {renderIcon(option.icon, isSelected)}
                            <span
                                className={`
                                    font-medium text-[11px] sm:text-xs text-left leading-5
                                    transition-colors duration-200
                                    ${isSelected
                                        ? "text-white"
                                        : "text-[var(--tailwind-colors-slate-400)] [:root:not(.dark)_&]:text-[var(--tailwind-colors-slate-500)]"
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

import { Switch } from "@/components/ui/switch";
import { type JSX } from "react";

interface BasicSwitchProps {
    className?: string;
}

export default function SwitchBase({ className }: BasicSwitchProps): JSX.Element {
    return (
        <Switch
            className={`
                w-9 h-5
                data-[state=unchecked]:bg-[var(--tailwind-colors-slate-700)]
                data-[state=checked]:bg-[var(--tailwind-colors-rdns-600)]
                [&>[data-slot=switch-thumb]]:bg-background
                data-[state=checked]:[&>[data-slot=switch-thumb]]:bg-[var(--tailwind-colors-slate-50)]
                data-[state=unchecked]:[&>[data-slot=switch-thumb]]:bg-[var(--tailwind-colors-slate-400)]
                ${className}
            `}
        />
    );
}
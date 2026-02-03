import type { JSX } from "react";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { Search, CheckSquare, XSquare } from "lucide-react";

interface CustomRulesSearchProps {
    value: string;
    onChange: (value: string) => void;
    allSelected: boolean;
    onSelectAll: () => void;
    onDeselectAll: () => void;
}

export default function CustomRulesSearch({
    value,
    onChange,
    allSelected,
    onSelectAll,
    onDeselectAll,
}: CustomRulesSearchProps): JSX.Element {
    return (
        <div className="flex gap-2.5 w-full">
            <div className="relative flex-1">
                <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-[var(--tailwind-colors-slate-400)]" />
                <Input
                    placeholder="Search a domain or IP address"
                    value={value}
                    onChange={(e) => onChange(e.target.value)}
                    className="pl-10 pr-3 py-2 !bg-[var(--shadcn-ui-app-background)] border-[var(--tailwind-colors-slate-700)] text-[var(--tailwind-colors-slate-50)] rounded-lg h-9 min-h-11 lg:min-h-0 placeholder:text-s md:placeholder:text-sm"
                    autoCapitalize="none"
                    spellCheck={false}
                    autoCorrect="off"
                />
            </div>

            {allSelected ? (
                <Button
                    variant="outline"
                    className="h-9 min-h-11 lg:min-h-0 px-2 py-1.5 bg-[var(--tailwind-colors-slate-800)] border-none text-[var(--tailwind-colors-rdns-600)] flex items-center"
                    onClick={onDeselectAll}
                    aria-label="Deselect all"
                >
                    <XSquare className="h-4 w-4" />
                    <span className="hidden md:inline ml-1">Deselect all</span>
                </Button>
            ) : (
                <Button
                    variant="outline"
                    className="w-11 md:w-auto h-9 min-h-11 lg:min-h-0 px-2 py-1.5 bg-[var(--tailwind-colors-slate-800)] border-none text-[var(--tailwind-colors-rdns-600)] flex items-center md:px-3"
                    onClick={onSelectAll}
                    aria-label="Select all"
                >
                    <CheckSquare className="h-4 w-4" />
                    <span className="hidden md:inline ml-1">Select all</span>
                </Button>
            )}
        </div>
    );
}
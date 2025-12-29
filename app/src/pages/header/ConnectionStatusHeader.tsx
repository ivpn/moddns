import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Separator } from "@/components/ui/separator";
import { type JSX, useState } from "react";
import { useAppStore } from "@/store/general";
import { useDnsConnectionStatus } from "@/hooks/useDnsConnectionStatus";

export default function ConnectionStatusHeader(): JSX.Element | null {
    const [isCollapsed, setIsCollapsed] = useState(false);
    const [isHiding, setIsHiding] = useState(false);
    const setConnectionStatusVisible = useAppStore((state) => state.setConnectionStatusVisible);

    const { status } = useDnsConnectionStatus(5000, { enabled: true });
    const { badge, message, messageColor } = status;

    if (isCollapsed) return null;

    const handleHide = () => {
        setIsHiding(true);
        setTimeout(() => {
            setIsCollapsed(true);
            setConnectionStatusVisible(false);
        }, 300);
    };

    return (
        <div
            data-testid="conn-header-root"
            className={`flex h-12 items-center gap-2.5 px-6 py-3 bg-[var(--variable-collection-surface)] w-full transition-all duration-300 ease-in-out overflow-hidden ${isHiding ? 'opacity-0 -translate-y-full max-h-0' : 'opacity-100 translate-y-0 max-h-12'}`}
        >
            <div data-testid="conn-header-label" className="font-bold text-[var(--tailwind-colors-slate-50)] text-xs leading-3 whitespace-nowrap font-['Roboto_Mono-Bold',Helvetica]">
                Status
            </div>
            <Badge data-testid="conn-header-badge" className={`${badge.className} text-[var(--tailwind-colors-slate-50)] px-2.5 py-0.5 rounded`}>
                <span data-testid="conn-header-badge-text" className="font-text-xs-leading-4-semibold text-xs font-semibold whitespace-nowrap">{badge.text}</span>
            </Badge>
            <Separator data-testid="conn-header-separator" orientation="vertical" className="h-5" />
            <div data-testid="conn-header-message" className={`font-normal ${messageColor} text-xs leading-6 whitespace-nowrap font-['Roboto_Flex-Regular',Helvetica]`}>{message}</div>
            <div className="flex-1" />
            <Button
                variant="ghost"
                className="h-auto min-w-16 p-1.5 mt-[-4.00px] mb-[-4.00px] rounded-[6px] cursor-pointer hover:bg-[var(--tailwind-colors-rdns-alpha-900)] transition-colors duration-200"
                onClick={handleHide}
            >
                <span data-testid="conn-header-hide" className="px-1 text-xs leading-5 font-medium text-[var(--tailwind-colors-rdns-600)]">Hide</span>
            </Button>
        </div>
    );
}

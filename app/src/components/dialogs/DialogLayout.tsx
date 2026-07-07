import React from "react";
import { cn } from "@/lib/utils";

// Shared body wrapper to provide consistent padding and scroll behavior on small screens.
export const DialogBody: React.FC<React.HTMLAttributes<HTMLDivElement>> = ({ className, ...props }) => (
    <div
        className={cn(
            "px-6 pb-4 pt-0 w-full overflow-y-auto",
            // Cap the body height (viewport minus an approximate header+footer reserve)
            // and keep it scrollable at ALL breakpoints. Previously this was
            // sm:max-h-none, which removed the cap on >=640px-wide screens and disabled
            // the body scroll — so on short-but-wide laptop viewports tall content (e.g.
            // a long import profile list + re-auth block) overflowed and the modal
            // header/footer were clipped off-screen with no way to scroll.
            "max-h-[calc(100dvh-12rem)]",
            className
        )}
        {...props}
    />
);

interface DialogActionsProps extends React.HTMLAttributes<HTMLDivElement> {
    /** When true, primary action appears first on mobile (stacked) */
    reverseStack?: boolean;
    /** Align actions to start instead of end (rare) */
    alignStart?: boolean;
}

// Standardized footer actions area.
export const DialogActions: React.FC<DialogActionsProps> = ({
    className,
    children,
    reverseStack = false,
    alignStart = false,
    ...props
}) => (
    <div
        className={cn(
            "flex flex-col sm:flex-row gap-3 px-6 pb-6 pt-0",
            alignStart ? "sm:justify-start" : "sm:justify-end",
            reverseStack && "flex-col-reverse sm:flex-row",
            className
        )}
        {...props}
    >
        {children}
    </div>
);

export default { DialogBody, DialogActions };

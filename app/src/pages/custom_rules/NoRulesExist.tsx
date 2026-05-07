import { FilterX } from "lucide-react";
import { type JSX } from "react";

interface NoRulesExistProps {
    type: "denied" | "allowed";
    title?: string;
    message?: string;
    showInput?: boolean;
    composer?: JSX.Element;
}

export default function NoRulesExist({
    type,
    title,
    message = "",
    showInput = false,
    composer,
}: NoRulesExistProps): JSX.Element {
    const defaultTitle = type === "denied" ? "There are no denied domains yet." : "There are no allowed domains yet.";

    return (
        // On mobile we want this block higher (no scroll). Remove vertical centering on mobile, keep it on md+.
        // Add a modest top margin and slightly tighter gap on mobile.
        <div className="flex flex-col w-full max-w-[551px] items-center md:justify-center gap-6 md:gap-8 relative mt-6 md:mt-0">
            <div className="inline-flex flex-col items-center gap-4 relative flex-[0_0_auto]">
                <div className="flex w-12 h-12 items-center justify-center gap-2.5 relative rounded-sm">
                    <FilterX className="!w-9 !h-9 !relative text-[var(--tailwind-colors-rdns-600)]" />
                </div>

                <div className="inline-flex flex-col justify-center gap-2 relative flex-[0_0_auto]">
                    <div className="inline-flex flex-col gap-2 relative flex-[0_0_auto]">
                        <div className="inline-flex flex-col items-start justify-center gap-2 relative flex-[0_0_auto]">
                            <div className="relative w-full max-w-sm mt-[-1.00px] font-text-lg-leading-7-semibold font-bold text-[var(--tailwind-colors-slate-50)] text-[length:var(--text-lg-leading-7-semibold-font-size)] text-center tracking-[var(--text-lg-leading-7-semibold-letter-spacing)] leading-[var(--text-lg-leading-7-semibold-line-height)] [font-style:var(--text-lg-leading-7-semibold-font-style)]">
                                {title ?? defaultTitle}
                            </div>
                        </div>

                        <div className="relative w-full max-w-sm [font-family:'Roboto_Flex-Regular',Helvetica] font-normal text-[var(--tailwind-colors-slate-100)] text-sm text-center tracking-[0] leading-5">
                            {message}
                        </div>
                    </div>
                </div>
            </div>

            {showInput && composer && (
                <div
                    className="flex flex-col md:flex-row items-stretch md:items-start gap-3 md:gap-2 relative self-stretch w-full"
                    data-testid="no-rules-input-wrapper"
                >
                    {composer}
                </div>
            )}
        </div>
    );
}

import { ExternalLink } from "lucide-react";

interface AuthFooterProps {
    variant?: "absolute" | "relative";
    openInNewTab?: boolean;
}

export default function AuthFooter({ variant = "absolute", openInNewTab = true }: AuthFooterProps) {
    const containerClass = variant === "absolute"
        ? "absolute bottom-4 left-1/2 transform -translate-x-1/2"
        : "mt-8 flex justify-center";

    const linkTarget = openInNewTab ? "_blank" : undefined;
    const linkRel = openInNewTab ? "noopener noreferrer" : undefined;
    const newTabHint = openInNewTab ? " (opens in new tab)" : "";

    return (
        <div className={containerClass}>
            <div className="flex items-center gap-4">
                <a
                    href="/tos"
                    target={linkTarget}
                    rel={linkRel}
                    className="text-sm !text-[var(--tailwind-colors-slate-100)] hover:!text-[var(--tailwind-colors-slate-200)] cursor-pointer transition-colors no-underline"
                    title={`Go to Terms of Service page${newTabHint}`}
                >
                    Terms of Service
                </a>
                <span className="text-sm text-[var(--tailwind-colors-slate-300)]">|</span>
                <a
                    href="/privacy"
                    target={linkTarget}
                    rel={linkRel}
                    className="text-sm !text-[var(--tailwind-colors-slate-100)] hover:!text-[var(--tailwind-colors-slate-200)] cursor-pointer transition-colors no-underline"
                    title={`Go to Privacy Policy page${newTabHint}`}
                >
                    Privacy Policy
                </a>
                <span className="text-sm text-[var(--tailwind-colors-slate-300)]">|</span>
                <a
                    href="/faq"
                    target={linkTarget}
                    rel={linkRel}
                    className="text-sm !text-[var(--tailwind-colors-slate-100)] hover:!text-[var(--tailwind-colors-slate-200)] cursor-pointer transition-colors no-underline"
                    title={`Go to FAQ page${newTabHint}`}
                >
                    FAQ
                </a>
                <span className="text-sm text-[var(--tailwind-colors-slate-300)]">|</span>
                <a
                    href="https://ivpn.net"
                    target="_blank"
                    rel="noopener noreferrer"
                    className="inline-flex items-center gap-1 text-sm !text-[var(--tailwind-colors-slate-100)] hover:!text-[var(--tailwind-colors-slate-200)] cursor-pointer transition-colors no-underline"
                    title="Go to IVPN website (opens in new tab)"
                >
                    IVPN
                    <ExternalLink className="w-3 h-3" />
                </a>
            </div>
        </div>
    );
}

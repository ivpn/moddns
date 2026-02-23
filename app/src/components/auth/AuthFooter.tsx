import { ExternalLink } from "lucide-react";
import { Link } from "react-router-dom";

interface AuthFooterProps {
    variant?: "absolute" | "relative";
    openInNewTab?: boolean;
}

const linkClass = "text-sm !text-[var(--tailwind-colors-slate-100)] hover:!text-[var(--tailwind-colors-slate-200)] cursor-pointer transition-colors no-underline";

interface InternalLinkProps {
    href: string;
    title: string;
    openInNewTab: boolean;
    children: React.ReactNode;
}

function InternalLink({ href, title, openInNewTab, children }: InternalLinkProps) {
    if (openInNewTab) {
        return (
            <a
                href={href}
                target="_blank"
                rel="noopener noreferrer"
                className={linkClass}
                title={`${title} (opens in new tab)`}
            >
                {children}
            </a>
        );
    }

    return (
        <Link to={href} className={linkClass} title={title}>
            {children}
        </Link>
    );
}

export default function AuthFooter({ variant = "absolute", openInNewTab = true }: AuthFooterProps) {
    const containerClass = variant === "absolute"
        ? "absolute bottom-4 left-1/2 transform -translate-x-1/2"
        : "mt-8 flex justify-center";

    return (
        <div className={containerClass}>
            <div className="flex items-center gap-4">
                <InternalLink href="/tos" title="Go to Terms of Service page" openInNewTab={openInNewTab}>
                    Terms of Service
                </InternalLink>
                <span className="text-sm text-[var(--tailwind-colors-slate-300)]">|</span>
                <InternalLink href="/privacy" title="Go to Privacy Policy page" openInNewTab={openInNewTab}>
                    Privacy Policy
                </InternalLink>
                <span className="text-sm text-[var(--tailwind-colors-slate-300)]">|</span>
                <InternalLink href="/faq" title="Go to FAQ page" openInNewTab={openInNewTab}>
                    FAQ
                </InternalLink>
                <span className="text-sm text-[var(--tailwind-colors-slate-300)]">|</span>
                <a
                    href="https://ivpn.net"
                    target="_blank"
                    rel="noopener noreferrer"
                    className={"inline-flex items-center gap-1 " + linkClass}
                    title="Go to IVPN website (opens in new tab)"
                >
                    IVPN
                    <ExternalLink className="w-3 h-3" />
                </a>
            </div>
        </div>
    );
}

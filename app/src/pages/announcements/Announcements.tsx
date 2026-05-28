import { type JSX, useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import ReactMarkdown from "react-markdown";
import { format } from "date-fns";
import {
    ArrowLeft,
    Newspaper,
    Sparkles,
    Wrench,
    AlertTriangle,
    ShieldAlert,
    ScrollText,
    ExternalLink,
    type LucideIcon,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { useTheme } from "@/components/theme-provider";
import modDNSLogoDarkTheme from "@/assets/logos/modDNS-dark-theme.svg";
import modDNSLogoLightTheme from "@/assets/logos/modDNS-light-theme.svg";
import AuthFooter from "@/components/auth/AuthFooter";
import api from "@/api/api";
import { useAppStore } from "@/store/general";
import {
    type AnnouncementsAnnouncement,
    AnnouncementsCategory,
    AnnouncementsSeverity,
} from "@/api/client";

const CATEGORY_META: Record<string, { label: string; Icon: LucideIcon }> = {
    [AnnouncementsCategory.CategoryNews]: { label: "News", Icon: Newspaper },
    [AnnouncementsCategory.CategoryFeature]: { label: "Feature", Icon: Sparkles },
    [AnnouncementsCategory.CategoryMaintenance]: { label: "Maintenance", Icon: Wrench },
    [AnnouncementsCategory.CategoryIncident]: { label: "Incident", Icon: AlertTriangle },
    [AnnouncementsCategory.CategorySecurity]: { label: "Security", Icon: ShieldAlert },
    [AnnouncementsCategory.CategoryPolicy]: { label: "Policy", Icon: ScrollText },
};

// Severity drives visual prominence: border accent + badge colour.
const SEVERITY_CLASSES: Record<string, { border: string; badge: string }> = {
    [AnnouncementsSeverity.SeverityInfo]: {
        border: "border-l-sky-500",
        badge: "bg-sky-500/10 text-sky-600 dark:text-sky-400",
    },
    [AnnouncementsSeverity.SeverityWarning]: {
        border: "border-l-amber-500",
        badge: "bg-amber-500/10 text-amber-600 dark:text-amber-400",
    },
    [AnnouncementsSeverity.SeverityCritical]: {
        border: "border-l-red-500",
        badge: "bg-red-500/10 text-red-600 dark:text-red-400",
    },
};

function formatDate(value?: string): string {
    if (!value) return "";
    const date = new Date(value);
    return isNaN(date.getTime()) ? "" : format(date, "PP");
}

function AnnouncementCard({ item }: { item: AnnouncementsAnnouncement }): JSX.Element {
    const severity = item.severity ?? AnnouncementsSeverity.SeverityInfo;
    const category = item.category ?? AnnouncementsCategory.CategoryNews;
    const sev = SEVERITY_CLASSES[severity] ?? SEVERITY_CLASSES[AnnouncementsSeverity.SeverityInfo];
    const cat = CATEGORY_META[category] ?? CATEGORY_META[AnnouncementsCategory.CategoryNews];
    const CatIcon = cat.Icon;
    const date = formatDate(item.published_at);

    return (
        <article
            className={`border border-[var(--shadcn-ui-app-border)] border-l-4 ${sev.border} rounded-lg p-5 bg-[var(--shadcn-ui-app-popover)]`}
        >
            <div className="flex flex-wrap items-center gap-3 mb-2">
                <span
                    className={`inline-flex items-center gap-1.5 rounded-full px-2.5 py-0.5 text-xs font-medium ${sev.badge}`}
                >
                    <CatIcon className="h-3.5 w-3.5" />
                    {cat.label}
                </span>
                {date && (
                    <time className="text-sm text-[var(--shadcn-ui-app-muted-foreground)]">{date}</time>
                )}
            </div>

            <h2 className="text-xl font-semibold text-[var(--shadcn-ui-app-foreground)] mb-2">
                {item.title}
            </h2>

            {item.body && (
                <div className="prose prose-sm dark:prose-invert max-w-none text-[var(--shadcn-ui-app-foreground)]">
                    <ReactMarkdown>{item.body}</ReactMarkdown>
                </div>
            )}

            {item.link && (
                <a
                    href={item.link}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="mt-3 inline-flex items-center gap-1.5 text-sm font-medium text-[var(--tailwind-colors-rdns-600)] hover:text-[var(--tailwind-colors-rdns-700)]"
                >
                    Learn more
                    <ExternalLink className="h-3.5 w-3.5" />
                </a>
            )}
        </article>
    );
}

export default function Announcements(): JSX.Element {
    const navigate = useNavigate();
    const { theme } = useTheme();
    const isDarkMode = theme === "dark";
    const canGoBack = typeof window !== "undefined" && window.history.length > 1;
    const markAnnouncementsSeen = useAppStore((s) => s.markAnnouncementsSeen);

    const [items, setItems] = useState<AnnouncementsAnnouncement[]>([]);
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState("");

    // Opening this page marks all current announcements as seen, clearing the
    // nav unread dot.
    useEffect(() => {
        markAnnouncementsSeen();
    }, [markAnnouncementsSeen]);

    useEffect(() => {
        let active = true;
        api.Client.announcementsApi
            .apiV1AnnouncementsGet()
            .then((resp) => {
                if (!active) return;
                setItems(resp.data ?? []);
                setError("");
            })
            .catch(() => {
                if (!active) return;
                setError("Unable to load announcements right now. Please try again later.");
            })
            .finally(() => {
                if (active) setLoading(false);
            });
        return () => {
            active = false;
        };
    }, []);

    return (
        <div className="relative min-h-screen w-full overflow-x-hidden bg-[var(--public-page-background)]">
            <div className="relative z-10 py-8">
                <div className="w-full max-w-4xl mx-auto p-8">
                    {canGoBack && (
                        <div className="mb-6">
                            <Button
                                variant="ghost"
                                onClick={() => navigate(-1)}
                                className="flex items-center gap-2 px-3 py-1.5 h-auto min-h-0 text-[var(--tailwind-colors-rdns-600)] hover:text-[var(--tailwind-colors-rdns-700)] hover:bg-black/5 dark:hover:bg-white/10 rounded-md"
                            >
                                <ArrowLeft className="h-4 w-4" />
                                Back
                            </Button>
                        </div>
                    )}

                    <Card className="bg-[var(--shadcn-ui-app-popover)] border-[var(--shadcn-ui-app-border)]">
                        <CardContent className="p-8">
                            <div className="flex flex-col items-center mb-8">
                                <img
                                    className="mb-4 w-[200px] h-10 mx-auto"
                                    alt="modDNS logo"
                                    src={isDarkMode ? modDNSLogoDarkTheme : modDNSLogoLightTheme}
                                />
                                <h1 className="text-2xl font-bold text-[var(--shadcn-ui-app-foreground)] text-center font-mono">
                                    Announcements
                                </h1>
                            </div>

                            {loading && (
                                <p className="text-center text-[var(--shadcn-ui-app-muted-foreground)]">
                                    Loading announcements…
                                </p>
                            )}

                            {!loading && error && (
                                <p className="text-center text-red-600 dark:text-red-400">{error}</p>
                            )}

                            {!loading && !error && items.length === 0 && (
                                <p className="text-center text-[var(--shadcn-ui-app-muted-foreground)]">
                                    No announcements at the moment.
                                </p>
                            )}

                            {!loading && !error && items.length > 0 && (
                                <div className="flex flex-col gap-4">
                                    {items.map((item) => (
                                        <AnnouncementCard key={item.id ?? item.title} item={item} />
                                    ))}
                                </div>
                            )}
                        </CardContent>
                    </Card>
                </div>
                <AuthFooter variant="relative" openInNewTab={false} />
            </div>
        </div>
    );
}

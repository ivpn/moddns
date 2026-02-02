import { Button } from "@/components/ui/button";
import { Card, CardContent, CardFooter } from "@/components/ui/card";
import {
    FileText,
    FilterX,
    Globe,
    LayoutList,
} from "lucide-react";
import { type JSX } from "react";
import modDNSLogoDarkTheme from '@/assets/logos/modDNS-dark-theme.svg'
import modDNSLogoLightTheme from '@/assets/logos/modDNS-light-theme.svg'
import { useNavigate } from "react-router-dom";
import { useTheme } from "@/components/theme-provider";
import { useAppStore } from '@/store/general';
import VerificationBanner from '@/pages/setup/VerificationBanner';

const Entry = (): JSX.Element => {
    const navigate = useNavigate();
    const { theme } = useTheme();
    const isDarkMode = theme === 'dark' || (theme === 'system' && typeof window !== 'undefined' && window.matchMedia('(prefers-color-scheme: dark)').matches);
    const account = useAppStore(state => state.account);
    const emailVerified = account?.email_verified;

    // Feature card data for mapping
    const featureCards = [
        {
            icon: <Globe className="h-[22px] w-[22px]" />,
            title: "Get started",
            description: [
                "Set up modDNS for your devices and browsers",
                "Android, iOS, Linux, macOS, Windows supported",
                "Custom setups using endpoint information",
            ],
            buttonText: "Setup DNS",
            route: "/setup",
            position: "col-start-1 row-start-1",
        },
        {
            icon: <LayoutList className="h-[22px] w-[22px]" />,
            title: "Set blocklists",
            description: [
                "Choose from dozens of popular DNS blocklists",
                "Combine lists or go with our recommendations",
                "Review blocklist sizes and update information",
            ],
            buttonText: "Add blocklists",
            route: "/blocklists",
            position: "col-start-2 row-start-1",
        },
        {
            icon: <FilterX className="h-[22px] w-[22px]" />,
            title: "Customise filtering",
            description: [
                "Add entries to your custom Allow or Deny list",
                "Override blocklist settings for better control",
                "Use domain or IP based custom rules",
            ],
            buttonText: "Set custom rules",
            route: "/custom-rules",
            position: "col-start-1 row-start-2",
        },
        {
            icon: <FileText className="h-[22px] w-[22px]" />,
            title: "Review DNS queries",
            description: [
                "Enable logs for query history (default - off)",
                "Set retention period for your data",
                "Filter and sort in query history",
            ],
            buttonText: "Check query logs",
            route: "/query-logs",
            position: "col-start-2 row-start-2",
        },
    ];

    return (
        <div className="flex flex-col min-h-screen">
            <div className="flex-1 p-6">
                <div className="max-w-6xl mx-auto">
                    <div className="flex flex-col items-center gap-12">
                        {/* Logo */}
                        <div className="inline-flex flex-col items-center gap-4 relative">
                            <img
                                className="w-full max-w-sm h-16 mx-auto"
                                alt="modDNS logo"
                                src={isDarkMode ? modDNSLogoDarkTheme : modDNSLogoLightTheme}
                                style={{ display: "block" }}
                            />
                        </div>
                        <div className="flex flex-col w-full max-w-2xl items-center gap-8">
                            <p className="text-center text-lg leading-7 text-[var(--shadcn-ui-app-muted-foreground)]">
                                <span className="text-[var(--shadcn-ui-app-muted-foreground)]">The </span>
                                <span className="font-semibold text-[var(--shadcn-ui-app-foreground)]">
                                    privacy-first
                                </span>
                                <span className="text-[var(--shadcn-ui-app-muted-foreground)]">
                                    {" "}
                                    DNS resolver in beta, developed by the team behind IVPN.
                                </span>
                            </p>
                            {/* Email verification warning banner (only if not verified & not dismissed) */}
                            {account && !emailVerified && (
                                <VerificationBanner emailVerified={emailVerified} />
                            )}
                        </div>

                        <div className="w-full max-w-4xl">
                            <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
                                {featureCards.map((card, index) => (
                                    <Card
                                        key={index}
                                        className="group flex flex-col items-start gap-8 p-6 bg-card border-[var(--shadcn-ui-app-border)] rounded-lg overflow-hidden hover:bg-accent transition-all duration-300 ease-in-out"
                                    >
                                        <CardContent className="flex flex-col items-start gap-4 p-0 w-full">
                                            <div className="inline-flex items-center gap-3">
                                                <div className="text-[var(--shadcn-ui-app-foreground)]">
                                                    {card.icon}
                                                </div>
                                                <h3 className="font-mono font-bold text-[var(--shadcn-ui-app-foreground)] text-lg leading-7 whitespace-nowrap">
                                                    {card.title}
                                                </h3>
                                            </div>

                                            <ul className="text-sm leading-relaxed text-[var(--tailwind-colors-slate-100)] group-hover:text-[var(--shadcn-ui-app-foreground)] list-disc list-inside space-y-1 transition-colors duration-300">
                                                {card.description.map((line, i) => (
                                                    <li key={i}>
                                                        {line}
                                                    </li>
                                                ))}
                                            </ul>
                                        </CardContent>

                                        <CardFooter className="p-0">
                                            <Button
                                                variant="outline"
                                                className="bg-muted text-[var(--tailwind-colors-rdns-600)] border-[var(--shadcn-ui-app-border)] hover:bg-[var(--tailwind-colors-rdns-600)] hover:text-primary-foreground group-hover:bg-[var(--tailwind-colors-rdns-600)] group-hover:text-primary-foreground dark:group-hover:text-white cursor-pointer transition-colors duration-300"
                                                onClick={() => navigate(card.route)}
                                            >
                                                {card.buttonText}
                                            </Button>
                                        </CardFooter>
                                    </Card>
                                ))}
                            </div>
                        </div>
                    </div>
                </div>
            </div>
        </div>
    );
};

export default Entry;

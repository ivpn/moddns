import {
    LogOutIcon,
    ShieldCheck,
    ShieldX,
    TrashIcon,
} from "lucide-react";
import type { JSX } from "react";
import { useState } from "react";
import { useLocation } from 'react-router-dom';
import { Button } from "@/components/ui/button";
import StatusBadge from "@/components/general/StatusBadge";
import { Card, CardContent } from "@/components/ui/card";
import AccountInfoCard from "@/pages/account_preferences/AccountInfoCard";
import type { ModelAccount } from "@/api/client/api";
import api from "@/api/api";
import ToggleGroup from "@/components/general/ToggleGroup";
import { toast } from "sonner"
import UpdatePasswordDialog from "@/pages/account_preferences/UpdatePasswordDialog";
import DeleteSessionsDialog from "@/pages/account_preferences/DeleteSessions";
import DeleteAccountDialog from "@/pages/account_preferences/DeleteAccount";
import Enable2FADialog from "@/pages/account_preferences/Enable2FA";
import Disable2FADialog from "@/pages/account_preferences/Disable2FA";
import PasskeySettings from "@/pages/account_preferences/PasskeySettings";
import VerifyEmailDialog from "@/pages/account_preferences/VerifyEmailDialog";
import ChangeEmailDialog from "@/pages/account_preferences/ChangeEmailDialog";
import { useAppStore } from "@/store/general";

interface PreferencesSectionProps {
    account: ModelAccount | null;
}

// Discriminated union for section items
interface BaseItem {
    name: string;
    description: string;
    hasToggle: boolean;
}
interface ToggleItem extends BaseItem {
    hasToggle: true;
    toggleOptions: { value: string; label: string; icon: string }[];
    toggleValue: string;
}
interface ActionItem extends BaseItem {
    hasToggle: false;
    action: string;
    actionIcon?: JSX.Element;
}
type SectionItem = ToggleItem | ActionItem;
interface SectionDef {
    title: string;
    items: SectionItem[];
}

const PreferencesSection = ({ account }: PreferencesSectionProps): JSX.Element => {
    // Local account state to allow refresh post-verification
    const [currentAccount, setCurrentAccount] = useState<ModelAccount | null>(account);
    const setAccount = useAppStore(s => s.setAccount);

    // State for error reports consent
    const [errorReportsConsent, setErrorReportsConsent] = useState(
        account?.error_reports_consent ? true : false
    );
    const [updatingConsent, setUpdatingConsent] = useState(false);
    const [showUpdatePassword, setShowUpdatePassword] = useState(false);
    const [showDeleteSessions, setShowDeleteSessions] = useState(false);
    // deletingSessions state removed (handled internally)
    const [showDeleteAccount, setShowDeleteAccount] = useState(false);
    const [showEnable2FA, setShowEnable2FA] = useState(false);
    const [showDisable2FA, setShowDisable2FA] = useState(false);
    const [showVerifyEmail, setShowVerifyEmail] = useState(false);
    const [showChangeEmail, setShowChangeEmail] = useState(false);
    const isEmailVerified = currentAccount?.email_verified;
    const location = useLocation();
    // --- highlightVerify state ---
    const [highlightVerify, setHighlightVerify] = useState(
        !isEmailVerified && new URLSearchParams(location.search).get('highlight') === 'verify'
    );

    // Add local state for 2FA enabled status
    const [is2FAEnabled, setIs2FAEnabled] = useState(currentAccount?.mfa?.totp?.enabled);

    // Handler to update 2FA state after enabling/disabling
    const handle2FAChanged = async (enabled: boolean) => {
        // Optionally, fetch updated account data from backend for full sync
        try {
            const accountData = await api.Client.accountsApi.apiV1AccountsCurrentGet();
            setCurrentAccount(accountData.data);
            setIs2FAEnabled(accountData.data.mfa?.totp?.enabled);
            // Update global store as well
            setAccount(accountData.data);
        } catch {
            // fallback to local state update if fetch fails
            setIs2FAEnabled(enabled);
        }
        setShowEnable2FA(false);
        setShowDisable2FA(false);
    };

    // Update error reports consent handler
    const handleErrorReportsConsentChange = async (value: boolean) => {
        setUpdatingConsent(true);
        try {
            await api.Client.accountsApi.apiV1AccountsPatch({
                updates: [
                    {
                        operation: "replace",
                        path: "/error_reports_consent",
                        // OpenAPI generator typed `value` as `object`; cast boolean accordingly
                        value: (value as unknown as object),
                    }
                ]
            });
            setErrorReportsConsent(value);
            toast.success("Error reports consent updated.");

        } catch {
            toast.error("Failed to update error reports consent.", {});
        } finally {
            setUpdatingConsent(false);
        }
    };

    // Account info data
    const accountInfo = [
        { label: "modDNS ID", value: currentAccount?.email || "" },
        {
            label: "Email status",
            value: (
                <StatusBadge
                    intent={currentAccount?.email_verified ? 'success' : 'error'}
                    text={currentAccount?.email_verified ? 'Verified' : 'Not verified'}
                    size="sm"
                    data-testid="email-status-badge"
                />
            ),
        },
        {
            label: "2FA",
            value: is2FAEnabled ? "Enabled" : "Disabled",
            icon: is2FAEnabled
                ? <ShieldCheck className="w-4 h-4 text-[var(--tailwind-colors-rdns-600)]" />
                : <ShieldX className="w-4 h-4 text-[var(--tailwind-colors-red-600)]" />,
        },
    ];

    // Section data
    const sections: SectionDef[] = [
        {
            title: "SECURITY",
            items: [
                {
                    name: "2FA",
                    description:
                        "When enabled, 2-factor authentication will be required when you log in.",
                    hasToggle: true,
                    toggleOptions: [
                        { value: "disable", label: "Disable", icon: "octagon-x" },
                        { value: "enable", label: "Enable", icon: "check" },
                    ],
                    toggleValue: is2FAEnabled ? "enable" : "disable",
                } as ToggleItem,
                {
                    name: "Email verification",
                    description: isEmailVerified ? "Your email is verified." : "Verify your email to enable account recovery and critical notifications.",
                    hasToggle: false,
                    action: "Verify email",
                } as ActionItem,
                {
                    name: "Change email",
                    description: "Update your email address.",
                    hasToggle: false,
                    action: "Change email",
                } as ActionItem,
                {
                    name: "Password",
                    description: "Update your password.",
                    hasToggle: false,
                    action: "Update password",
                } as ActionItem,
            ],
        },
        {
            title: "PRIVACY",
            items: [
                {
                    name: "Error reports",
                    description:
                        "No reports are sent to our servers by default. Enabling error reports sharing helps us improve modDNS.",
                    hasToggle: true,
                    toggleOptions: [
                        { value: "disable", label: "Disable", icon: "octagon-x" },
                        { value: "enable", label: "Enable", icon: "check" },
                    ],
                    toggleValue: errorReportsConsent ? "enable" : "disable",
                } as ToggleItem,
            ],
        },
        {
            title: "ACTIVE SESSIONS",
            items: [
                {
                    name: "Log out web sessions",
                    description:
                        "You can log out sessions initiated on other devices. This action won't affect your current session, endpoint setups, DNS resolutions and other settings.",
                    hasToggle: false,
                    action: "Log out web sessions",
                    actionIcon: <LogOutIcon className="w-4 h-4" />,
                } as ActionItem,
            ],
        },
    ];

    return (
        <section className="flex flex-col w-full gap-10 p-4 sm:p-8 overflow-x-hidden">
            <div className="flex flex-col gap-5 w-full">
                <div className="flex items-center justify-between w-full">
                    <div className="flex flex-col w-full max-w-[572px] items-start gap-1 px-1">
                    </div>
                </div>
            </div>

            <div className="flex flex-col gap-6">
                {/* Account Info Card */}
                <div className="flex flex-col gap-6 w-full">
                    <div
                        className="flex w-full justify-start md:max-w-[572px] lg:max-w-[640px] md:px-0"
                        data-testid="account-info-card-wrapper"
                    >
                        <AccountInfoCard accountInfo={accountInfo} />
                    </div>
                </div>

                {/* Sections */}
                {sections.map((section, sectionIndex) => (
                    <Card key={sectionIndex} className="w-full border-none">
                        <CardContent>
                            <div className="flex flex-col items-start gap-6 w-full">
                                <div className="flex items-center gap-2 w-full">
                                    <div className="flex flex-col items-start gap-2">
                                        <div className="[font-family:'Roboto_Mono-Bold',Helvetica] font-bold text-[var(--tailwind-colors-rdns-600)] text-base tracking-[0] leading-4">
                                            {section.title}
                                        </div>
                                    </div>
                                </div>

                                {section.items.map((item, itemIndex) => (
                                    <div
                                        key={itemIndex}
                                        className="flex flex-col sm:flex-row sm:items-center sm:justify-between w-full gap-3 sm:gap-4 max-w-full"
                                    >
                                        <div className="flex flex-col items-start gap-2 min-w-0 max-w-full">
                                            <div className="[font-family:'Roboto_Flex-Medium',Helvetica] font-bold text-[var(--tailwind-colors-slate-50)] text-base tracking-[0] leading-4 break-words">
                                                {item.name}
                                            </div>
                                            {item.description && (
                                                <div className="font-text-sm-leading-5-normal font-[number:var(--text-sm-leading-5-normal-font-weight)] text-[var(--tailwind-colors-slate-200)] text-[length:var(--text-sm-leading-5-normal-font-size)] tracking-[var(--text-sm-leading-5-normal-letter-spacing)] leading-[var(--text-sm-leading-5-normal-line-height)] [font-style:var(--text-sm-leading-5-normal-font-style)] break-words">
                                                    {item.description}
                                                </div>
                                            )}
                                        </div>

                                        {item.hasToggle ? (
                                            // Toggle items
                                            item.name === "2FA" ? (
                                                <ToggleGroup
                                                    options={item.toggleOptions}
                                                    value={is2FAEnabled ? "enable" : "disable"}
                                                    onChange={(val: string) => {
                                                        if (val === "enable" && !is2FAEnabled) setShowEnable2FA(true);
                                                        if (val === "disable" && is2FAEnabled) setShowDisable2FA(true);
                                                    }}
                                                    variant="outline"
                                                    className="rounded p-0.5"
                                                />
                                            ) : item.name === "Error reports" ? (
                                                <ToggleGroup
                                                    options={item.toggleOptions}
                                                    value={errorReportsConsent ? "enable" : "disable"}
                                                    onChange={(val: string) => !updatingConsent && handleErrorReportsConsentChange(val === "enable")}
                                                    variant="outline"
                                                    className="rounded p-0.5"
                                                />
                                            ) : (
                                                <ToggleGroup
                                                    options={item.toggleOptions}
                                                    value={item.toggleValue}
                                                    onChange={() => { }}
                                                    variant="outline"
                                                    className="rounded p-0.5"
                                                />
                                            )
                                        ) : (
                                            // Action items
                                            item.name === "Email verification" && item.action === "Verify email" ? (
                                                <Button
                                                    className={`h-auto min-h-11 lg:min-h-0 bg-[var(--tailwind-colors-rdns-600)] hover:bg-[var(--tailwind-colors-slate-800)] text-[var(--tailwind-colors-slate-800)] hover:text-[var(--tailwind-colors-rdns-600)] w-full sm:w-auto ${highlightVerify ? 'animate-pulse ring-2 ring-[var(--tailwind-colors-amber-400)]' : ''}`}
                                                    onClick={() => setShowVerifyEmail(true)}
                                                    disabled={isEmailVerified}
                                                >
                                                    <span className="text-sm break-words">{isEmailVerified ? 'Verified' : 'Verify email'}</span>
                                                </Button>
                                            ) : item.name === "Change email" && item.action === "Change email" ? (
                                                <Button
                                                    className="h-auto min-h-11 lg:min-h-0 bg-[var(--tailwind-colors-rdns-600)] hover:bg-[var(--tailwind-colors-slate-800)] text-[var(--tailwind-colors-slate-800)] hover:text-[var(--tailwind-colors-rdns-600)] w-full sm:w-auto"
                                                    onClick={() => setShowChangeEmail(true)}
                                                >
                                                    <span className="text-sm break-words">Change email</span>
                                                </Button>
                                            ) : item.action === "Update password" ? (
                                                <>
                                                    <Button
                                                        className="h-auto min-h-11 lg:min-h-0 bg-[var(--tailwind-colors-rdns-600)] hover:bg-[var(--tailwind-colors-slate-800)] text-[var(--tailwind-colors-slate-800)] hover:text-[var(--tailwind-colors-rdns-600)] w-full sm:w-auto"
                                                        onClick={() => setShowUpdatePassword(true)}
                                                    >
                                                        {item.actionIcon}
                                                        <span className="text-sm break-words text-left sm:text-center">{item.action}</span>
                                                    </Button>
                                                    {showUpdatePassword && (
                                                        <UpdatePasswordDialog open={showUpdatePassword} onOpenChange={setShowUpdatePassword} />
                                                    )}
                                                </>
                                            ) : item.action === "Log out web sessions" ? (
                                                <>
                                                    <Button
                                                        className="h-auto min-h-11 lg:min-h-0 bg-[var(--tailwind-colors-slate-800)] hover:bg-[var(--tailwind-colors-rdns-600)] text-[var(--tailwind-colors-rdns-600)] hover:text-[var(--tailwind-colors-slate-800)] w-full sm:w-auto"
                                                        onClick={() => setShowDeleteSessions(true)}
                                                    >
                                                        {item.actionIcon}
                                                        <span className="text-sm break-words text-left sm:text-center">{item.action}</span>
                                                    </Button>
                                                    <DeleteSessionsDialog
                                                        open={showDeleteSessions}
                                                        onOpenChange={setShowDeleteSessions}
                                                    />
                                                </>
                                            ) : null
                                        )}
                                    </div>
                                ))}
                            </div>
                        </CardContent>
                    </Card>
                ))}

                {/* Passkey Management Section */}
                <PasskeySettings />

                {/* Delete Account Section */}
                <Card className="w-full border-none bg-[var(--tailwind-colors-red-950)] rounded-[var(--primitives-radius-radius)]">
                    <CardContent className="bg-transparent">
                        <div className="flex flex-col items-start gap-6 w-full">
                            <div className="flex flex-col sm:flex-row sm:items-center justify-between w-full gap-4 flex-wrap">
                                <div className="flex flex-col items-start gap-2 flex-shrink min-w-0 max-w-full">
                                    <div className="[font-family:'Roboto_Flex-Medium',Helvetica] font-bold text-[var(--tailwind-colors-slate-50)] text-base tracking-[0] leading-4 whitespace-nowrap">
                                        Delete account
                                    </div>
                                    <div className="font-text-sm-leading-5-normal font-[number:var(--text-sm-leading-5-normal-font-weight)] text-[var(--tailwind-colors-base-white)] text-[length:var(--text-sm-leading-5-normal-font-size)] tracking-[var(--text-sm-leading-5-normal-letter-spacing)] leading-[var(--text-sm-leading-5-normal-line-height)] [font-style:var(--text-sm-leading-5-normal-font-style)]">
                                        When you delete your account we immediately remove all
                                        associated data including all profile information,
                                        blocklists and custom settings.
                                    </div>
                                </div>

                                <div className="mt-2 sm:mt-0 flex items-start sm:items-center">
                                    <Button
                                        className="bg-[var(--tailwind-colors-red-600)] text-[var(--tailwind-colors-slate-50)] h-auto min-h-11 lg:min-h-0 flex items-center gap-1 w-full sm:w-auto"
                                        onClick={() => setShowDeleteAccount(true)}
                                    >
                                        <TrashIcon className="w-4 h-4" />
                                        <span className="text-sm break-words">Delete account</span>
                                    </Button>
                                </div>
                                <DeleteAccountDialog
                                    open={showDeleteAccount}
                                    onOpenChange={setShowDeleteAccount}
                                />
                            </div>
                        </div>
                    </CardContent>
                </Card>

                <Enable2FADialog
                    open={showEnable2FA}
                    onOpenChange={open => setShowEnable2FA(open)}
                    onEnabled={() => handle2FAChanged(true)}
                />
                <Disable2FADialog
                    open={showDisable2FA}
                    onOpenChange={open => setShowDisable2FA(open)}
                    onDisabled={() => handle2FAChanged(false)}
                />
                <VerifyEmailDialog
                    open={showVerifyEmail}
                    onOpenChange={setShowVerifyEmail}
                    email={currentAccount?.email || ""}
                    onVerified={() => {
                        // Re-fetch account to update UI (badge, banner, button)
                        (async () => {
                            try {
                                const refreshed = await api.Client.accountsApi.apiV1AccountsCurrentGet();
                                setCurrentAccount(refreshed.data);
                                // Update global store as well
                                setAccount(refreshed.data);
                            } finally {
                                setShowVerifyEmail(false);
                            }
                        })();
                    }}
                />
                <ChangeEmailDialog
                    open={showChangeEmail}
                    onOpenChange={setShowChangeEmail}
                    account={currentAccount}
                    onChanged={() => {
                        // After successful email change we already updated global store; refresh local copy
                        (async () => {
                            try {
                                const refreshed = await api.Client.accountsApi.apiV1AccountsCurrentGet();
                                setCurrentAccount(refreshed.data);
                                // Update global store as well
                                setAccount(refreshed.data);
                            } finally {
                                setShowChangeEmail(false);
                                setShowVerifyEmail(false); // ensure verify dialog closed if open
                                setHighlightVerify(true); // highlight Verify button after email change
                            }
                        })();
                    }}
                />
            </div>
        </section >
    );
};

export default PreferencesSection;

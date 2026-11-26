import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import {
    Dialog,
    DialogContent,
    DialogHeader,
    DialogTitle,
} from "@/components/ui/dialog";
import { ClipboardIcon, CheckIcon, Loader2 } from "lucide-react";
import { useState, useEffect } from "react";
import QRCode from "react-qr-code";
import { toast } from "sonner";
import api from "@/api/api";
import { useAppStore } from "@/store/general";
import BackupCodesView from "@/components/dialogs/BackupCodesView";
import TwoFactorCodeInput from "@/components/TwoFactorCodeInput";
import { totpErrorMessage } from "@/lib/authToasts";

interface TwoFactorAuthDialogProps {
    open: boolean;
    onOpenChange: (open: boolean) => void;
    onEnabled?: () => void;
}

export default function TwoFactorAuthDialog({ open, onOpenChange, onEnabled }: TwoFactorAuthDialogProps) {
    const [copied, setCopied] = useState(false);
    const [loading, setLoading] = useState(false);
    const [secret, setSecret] = useState<string>("");
    const [uri, setUri] = useState<string>("");
    const [otp, setOtp] = useState<string>("");
    const [otpValid, setOtpValid] = useState(false);
    const [backupCodes, setBackupCodes] = useState<string[] | null>(null);
    const [enabling, setEnabling] = useState(false);
    const [visible, setVisible] = useState(open);
    const [backupCodesConfirmed, setBackupCodesConfirmed] = useState(false);
    const setAccount = useAppStore(s => s.setAccount);

    useEffect(() => {
        if (open) {
            setVisible(true);
            setLoading(true);
            setBackupCodes(null);
            setBackupCodesConfirmed(false);
            setOtp(""); // Clear OTP when opening dialog
            setOtpValid(false);
            api.Client.accountsApi.apiV1AccountsMfaTotpEnablePost()
                .then(res => {
                    setSecret(res.data.secret || "");
                    setUri(res.data.uri || "");
                })
                .catch(() => {
                    toast.error("Failed to fetch 2FA setup data.");
                    onOpenChange(false);
                })
                .finally(() => setLoading(false));
        } else {
            // Clear OTP when closing dialog
            setOtp("");
            // Wait for fade-out before hiding dialog
            const timeout = setTimeout(() => setVisible(false), 200); // 200ms matches Tailwind duration-200
            return () => clearTimeout(timeout);
        }
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [open]);

    const handleCopy = async () => {
        try {
            await navigator.clipboard.writeText(secret);
            setCopied(true);
            toast.success("Secret copied to clipboard");
            setTimeout(() => setCopied(false), 1200);
        } catch {
            toast.error("Failed to copy");
        }
    };

    const handleEnable2FA = async () => {
        setEnabling(true);
        try {
            const body = { otp } as unknown as { otp: string };
            const response = await api.Client.accountsApi.apiV1AccountsMfaTotpEnableConfirmPost(body);
            setBackupCodes(response.data.backup_codes || []);
            setOtp(""); // Clear OTP after successful confirmation
            toast.success("2FA enabled successfully.");
            // Do NOT call onEnabled here, wait for user confirmation
        } catch (e: unknown) {
            const axiosErr = e as { response?: { data?: { error?: string } } };
            toast.error(totpErrorMessage(axiosErr?.response?.data?.error, "Failed to enable 2FA."));
        } finally {
            setEnabling(false);
        }
    };

    // Handler for confirming backup codes
    const handleBackupCodesConfirmed = async () => {
        setBackupCodesConfirmed(true);

        // Update the global account state with latest 2FA status
        try {
            const refreshed = await api.Client.accountsApi.apiV1AccountsCurrentGet();
            setAccount(refreshed.data);
        } catch (error) {
            // Log error but don't block the flow
            console.error('Failed to refresh account after enabling 2FA:', error);
        }

        if (onEnabled) onEnabled();
        onOpenChange(false);
    };

    return (
        <Dialog
            open={visible}
            onOpenChange={openVal => {
                // Prevent closing by outside click if backup codes are shown and not confirmed
                if (backupCodes && !backupCodesConfirmed && openVal === false) return;
                onOpenChange(openVal);
            }}
            // Prevent closing with Escape key if backup codes are shown and not confirmed
            modal={true}
        >
            <DialogContent
                className={`
                    w-[95vw] sm:max-w-2xl max-h-[90dvh] overflow-y-auto overscroll-contain p-0 border-[var(--tailwind-colors-slate-600)]
                    bg-[var(--shadcn-ui-app-background)] 
                    [&_[data-slot=dialog-close]_svg]:text-[var(--tailwind-colors-rdns-600)]
                    transition-opacity duration-200
                    ${open ? "opacity-100" : "opacity-0 pointer-events-none"}
                `}
                // Prevent closing by clicking outside or pressing Escape
                onInteractOutside={e => {
                    if (backupCodes && !backupCodesConfirmed) e.preventDefault();
                }}
                onEscapeKeyDown={e => {
                    if (backupCodes && !backupCodesConfirmed) e.preventDefault();
                }}
            >
                <DialogHeader className="p-4 sm:p-6 pb-0 sm:pb-0">
                    <DialogTitle className="text-lg font-semibold text-[var(--tailwind-colors-slate-50)] tracking-[-0.45px] leading-[18px] font-['Roboto_Flex-SemiBold',Helvetica]">
                        Enable 2-Factor Authentication
                    </DialogTitle>
                </DialogHeader>

                <div className="flex flex-col items-start gap-8 p-4 sm:p-6">
                    {backupCodes ? (
                        <BackupCodesView codes={backupCodes} onDone={handleBackupCodesConfirmed} />
                    ) : (
                        <>
                            <div className="flex flex-col items-start gap-4 pb-8 w-full border-b border-[var(--tailwind-colors-slate-600)]">
                                <p className="text-sm font-normal text-[var(--tailwind-colors-slate-50)] tracking-[-0.35px] leading-[19.6px] font-['Roboto_Flex-Regular',Helvetica]">
                                    To enable two-factor authentication, please scan the code with a TOTP app (for example: Google Authenticator) and enter the code in the field below.
                                    <br /><br />
                                    If you cannot scan QR code, you can enter the following information manually.
                                </p>

                                <Card className="w-full bg-[var(--tailwind-colors-slate-900)] p-4">
                                    <div className="flex flex-col items-start gap-6 w-full">
                                        <div className="w-full flex justify-center px-2">
                                            {uri ? (
                                                <div className="max-w-full">
                                                    <QRCode
                                                        value={uri}
                                                        fgColor="var(--tailwind-colors-rdns-600)"
                                                        bgColor="transparent"
                                                        size={160}
                                                        className="w-full h-auto max-w-[160px] max-h-[160px]"
                                                    />
                                                </div>
                                            ) : (
                                                <div className="h-[128px]" />
                                            )}
                                        </div>

                                        <p className="w-full text-sm font-normal text-[var(--tailwind-colors-slate-100)] tracking-[-0.35px] leading-[19.6px] text-center font-['Roboto_Flex-Regular',Helvetica]">
                                            or enter the following code in your authenticator app:
                                        </p>

                                        <div className="flex items-center gap-2 w-full min-w-0">
                                            <div className="flex-1 text-xs sm:text-sm font-bold text-[var(--tailwind-colors-slate-50)] text-center leading-tight font-['Roboto_Mono-Bold',Helvetica] overflow-hidden break-all min-w-0 px-1">
                                                {secret}
                                            </div>

                                            <Button
                                                variant="ghost"
                                                size="icon"
                                                className="text-[var(--tailwind-colors-rdns-600)] bg-[var(--tailwind-colors-slate-900)] rounded-md h-auto p-1.5 flex-shrink-0"
                                                onClick={handleCopy}
                                                onMouseLeave={() => setCopied(false)}
                                                disabled={!secret}
                                            >
                                                {copied ? (
                                                    <CheckIcon className="w-4 h-4 text-[var(--tailwind-colors-rdns-600)]" />
                                                ) : (
                                                    <ClipboardIcon className="w-4 h-4" />
                                                )}
                                            </Button>
                                        </div>
                                    </div>
                                </Card>
                            </div>

                            <div className="flex flex-col items-start gap-2 w-full">
                                <TwoFactorCodeInput
                                    id="totp-code"
                                    value={otp}
                                    onChange={(value, valid) => { setOtp(value); setOtpValid(valid); }}
                                    onSubmit={() => {
                                        if (!loading && !enabling && !!secret) handleEnable2FA();
                                    }}
                                    disabled={loading || enabling}
                                    allowBackupCode={false}
                                    data-testid="enable-2fa-code"
                                />
                            </div>
                        </>
                    )}
                </div>

                {!backupCodes && (
                    <div className="flex flex-col sm:flex-row items-stretch sm:items-center justify-end gap-3 pt-0 pr-6 pb-6 pl-6">
                        <Button
                            variant="cancel"
                            size="lg"
                            className="flex-1 min-w-32 font-medium"
                            onClick={() => onOpenChange(false)}
                            disabled={enabling}
                        >
                            Cancel
                        </Button>
                        <Button
                            variant="default"
                            size="lg"
                            className="flex-1 min-w-32 bg-[var(--tailwind-colors-rdns-600)] text-[var(--tailwind-colors-slate-900)] hover:!bg-[var(--tailwind-colors-rdns-800)] flex items-center justify-center"
                            disabled={loading || !secret || !otpValid || enabling}
                            onClick={handleEnable2FA}
                        >
                            {enabling && <Loader2 className="animate-spin w-4 h-4 mr-2" />}
                            Enable 2FA
                        </Button>
                    </div>
                )}
            </DialogContent>
        </Dialog>
    );
}

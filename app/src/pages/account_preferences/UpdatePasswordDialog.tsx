import { useEffect, useMemo, useState, type JSX } from "react";
import { useAppStore } from '@/store/general';
import { Button } from "@/components/ui/button";
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogHeader,
    DialogTitle,
} from "@/components/ui/dialog";
import { DialogActions } from "@/components/dialogs/DialogLayout";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { toast } from "sonner";
import api from "@/api/api";
import { ModelAccountUpdateOperationEnum, ModelAccountUpdatePathEnum, type ModelAccountUpdate } from "@/api/client/api";

export default function UpdatePasswordDialog({ open, onOpenChange }: { open: boolean; onOpenChange: (open: boolean) => void; }): JSX.Element {
    const [oldPassword, setOldPassword] = useState("");
    const [newPassword, setNewPassword] = useState("");
    const [confirmPassword, setConfirmPassword] = useState("");
    const [loading, setLoading] = useState(false);
    const [closing, setClosing] = useState(false);
    const [otp, setOtp] = useState("");
    const account = useAppStore(s => s.account);
    const setAccount = useAppStore(s => s.setAccount);
    const passwordAvailable = useMemo(() => {
        if (!account) return true;
        const methods = account.auth_methods;
        if (!Array.isArray(methods) || methods.length === 0) {
            return true;
        }
        return methods.includes("password");
    }, [account]);
    const is2FAEnabled = account?.mfa?.totp?.enabled;

    useEffect(() => {
        if (!passwordAvailable) {
            setOldPassword("");
        }
    }, [passwordAvailable]);

    // Handle password update with optional 2FA (old password required only if available)
    const handleSubmit = async () => {
        if (passwordAvailable && !oldPassword) {
            toast.error("Old password is required.");
            return;
        }
        if (!newPassword || newPassword !== confirmPassword) {
            toast.error("Passwords do not match.");
            return;
        }

        if (is2FAEnabled && !otp) {
            toast.error('2FA code is required.');
            return;
        }

        setLoading(true);
        const updates: ModelAccountUpdate[] = [];
        if (passwordAvailable) {
            updates.push({
                operation: ModelAccountUpdateOperationEnum.Test,
                path: ModelAccountUpdatePathEnum.Password,
                value: oldPassword as unknown as object,
            });
        }
        updates.push({
            operation: ModelAccountUpdateOperationEnum.Replace,
            path: ModelAccountUpdatePathEnum.Password,
            value: newPassword as unknown as object,
        });
        const req: { updates: ModelAccountUpdate[] } = {
            updates,
        };
        try {
            // If 2FA is enabled, pass the OTP; otherwise, call without it
            if (is2FAEnabled) {
                await api.Client.accountsApi.apiV1AccountsPatch(req, otp, ['totp']);
            } else {
                await api.Client.accountsApi.apiV1AccountsPatch(req);
            }
            const refreshed = await api.Client.accountsApi.apiV1AccountsCurrentGet();
            setAccount(refreshed.data);
            toast.success("Password updated successfully.");
            handleClose();
        } catch (e: unknown) {
            const axiosErr = e as { response?: { data?: { error?: string } } };
            let errorMessage = axiosErr.response?.data?.error || "Failed to update password.";
            if (axiosErr.response?.data?.error === "password does not meet complexity requirements") {
                errorMessage = "Password must be 12-64 characters, contain at least one uppercase letter, one lowercase letter, one number, and one special character.";
            }
            toast.error(errorMessage);
        } finally {
            setLoading(false);
        }
    };

    // Subtle fade-out effect before closing dialog
    const handleClose = () => {
        setClosing(true);
        setTimeout(() => {
            setClosing(false);
            onOpenChange(false);
            setOtp("");
            setOldPassword("");
            setNewPassword("");
            setConfirmPassword("");
        }, 300); // Duration matches transition
    };

    return (
        <Dialog open={open} onOpenChange={(open) => open ? onOpenChange(open) : handleClose()}>
            <DialogContent
                className={`dialog-shell bg-[var(--shadcn-ui-app-background)] border-[var(--tailwind-colors-slate-600)] p-0 px-4 sm:px-0
                [&_[data-slot=dialog-close]_svg]:text-[var(--tailwind-colors-rdns-600)]
                transition-all duration-300 ease-out ${closing ? "opacity-0 scale-95" : "opacity-100 scale-100"} `}
            >
                <DialogHeader className="p-6 space-y-1.5">
                    <DialogTitle className="text-lg font-semibold text-[var(--tailwind-colors-slate-50)] tracking-[-0.45px] leading-[18px] font-['Roboto_Flex-SemiBold',Helvetica] mt-[-1px]">
                        Update password
                    </DialogTitle>
                    <DialogDescription className="text-sm font-normal text-[var(--tailwind-colors-slate-400)] font-['Roboto_Flex-Regular',Helvetica] leading-5">
                        Enter your new password below.
                    </DialogDescription>
                </DialogHeader>

                <div className="flex flex-col gap-4 px-6 pb-2">
                    {passwordAvailable && (
                        <div className="space-y-2">
                            <Label
                                htmlFor="old-password"
                                className="text-sm text-[var(--tailwind-colors-slate-50)] font-medium font-text-sm-leading-5-medium"
                            >
                                Old password
                            </Label>
                            <Input
                                id="old-password"
                                type="password"
                                value={oldPassword}
                                autoComplete="current-password"
                                onChange={e => setOldPassword(e.target.value)}
                                className="border-[var(--tailwind-colors-slate-700)] text-[var(--tailwind-colors-slate-50)] h-10"
                            />
                        </div>
                    )}
                    <div className="space-y-2">
                        <Label
                            htmlFor="new-password"
                            className="text-sm text-[var(--tailwind-colors-slate-50)] font-medium font-text-sm-leading-5-medium"
                        >
                            New password
                        </Label>
                        <Input
                            id="new-password"
                            type="password"
                            value={newPassword}
                            onChange={e => setNewPassword(e.target.value)}
                            className="border-[var(--tailwind-colors-slate-700)] text-[var(--tailwind-colors-slate-50)] h-10"
                        />
                    </div>

                    <div className="space-y-2">
                        <Label
                            htmlFor="confirm-password"
                            className="text-sm text-[var(--tailwind-colors-slate-50)] font-medium font-text-sm-leading-5-medium"
                        >
                            Confirm password
                        </Label>
                        <Input
                            id="confirm-password"
                            type="password"
                            value={confirmPassword}
                            onChange={e => setConfirmPassword(e.target.value)}
                            className="border-[var(--tailwind-colors-slate-700)] text-[var(--tailwind-colors-slate-50)] h-10"
                        />
                    </div>
                    {is2FAEnabled && (
                        <div className="space-y-2">
                            <Label htmlFor="otp-code" className="text-sm text-[var(--tailwind-colors-slate-50)] font-medium">2FA code</Label>
                            <Input
                                id="otp-code"
                                value={otp}
                                onChange={e => setOtp(e.target.value)}
                                onKeyDown={e => {
                                    if (e.key === 'Enter') {
                                        handleSubmit();
                                    }
                                }}
                                className="border-[var(--tailwind-colors-slate-700)] text-[var(--tailwind-colors-slate-50)] h-10"
                            />
                            <p className="text-xs text-[var(--tailwind-colors-slate-400)]">6-digit code</p>
                        </div>
                    )}
                    <p className="text-xs text-[var(--tailwind-colors-slate-400)]">After changing your password, you will need to use the new password to log in next time.</p>
                </div>

                <DialogActions>
                    <Button
                        variant="cancel"
                        size="lg"
                        className="flex-1 min-w-32 font-medium"
                        onClick={handleClose}
                        disabled={loading}
                    >
                        Cancel
                    </Button>
                    <Button
                        variant="default"
                        size="lg"
                        className="flex-1 min-w-32 bg-[var(--tailwind-colors-rdns-600)] text-[var(--tailwind-colors-slate-900)] hover:!bg-[var(--tailwind-colors-rdns-800)]"
                        onClick={handleSubmit}
                        disabled={loading}
                    >
                        {loading ? 'Updating...' : 'Save change'}
                    </Button>
                </DialogActions>
            </DialogContent>
        </Dialog >
    );
}

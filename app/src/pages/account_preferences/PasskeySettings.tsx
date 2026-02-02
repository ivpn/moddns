import { useState, useEffect } from "react";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { AlertCircle, Key, Plus, Trash2 } from "lucide-react";
import { toast } from "sonner";
import api from "@/api/api";
import { addPasskeyToAccount, isWebAuthnSupported } from "@/lib/webauthn";
import DeletePasskeyDialog from "./DeletePasskeyDialog";
import { useAppStore } from "@/store/general";
import type { ModelCredential } from "@/api/client/api";

export default function PasskeySettings() {
    const passkeys = useAppStore(state => state.passkeys);
    const setPasskeys = useAppStore(state => state.setPasskeys);
    const [loading, setLoading] = useState(false);
    const [registering, setRegistering] = useState(false);
    const [deletingId, setDeletingId] = useState<string | null>(null);
    const [webAuthnSupported, setWebAuthnSupported] = useState(false);

    // Dialog state
    const [deleteDialogOpen, setDeleteDialogOpen] = useState(false);
    const [passkeyToDelete, setPasskeyToDelete] = useState<{ id: string; name: string } | null>(null);

    useEffect(() => {
        // Check WebAuthn support
        setWebAuthnSupported(isWebAuthnSupported());
        loadPasskeys();
    // eslint-disable-next-line react-hooks/exhaustive-deps
    }, []);

    const loadPasskeys = async () => {
        try {
            setLoading(true);
            const response = await api.Client.authApi.apiV1WebauthnPasskeysGet();

            if (response.data && Array.isArray(response.data)) {
                setPasskeys(response.data as ModelCredential[]);
            } else {
                setPasskeys([]);
            }
        } catch (err) {
            // Narrow error shape cautiously
            const e = err as { message?: string };
            console.error('Failed to load passkeys:', e?.message || err);
            toast.error("Failed to load passkeys");
            setPasskeys([]); // Set empty array on error
        } finally {
            setLoading(false);
        }
    };

    const handleAddPasskey = async () => {
        try {
            setRegistering(true);
            await addPasskeyToAccount();
            toast.success("Passkey added successfully.");

            // Reload passkeys list
            await loadPasskeys();
        } catch (err) {
            const e = err as { message?: string };
            toast.error(e?.message || "Failed to add passkey");
        } finally {
            setRegistering(false);
        }
    };

    const handleDeletePasskey = async (passkeyId: string, passkeyName?: string) => {
        // Show confirmation dialog
        setPasskeyToDelete({
            id: passkeyId,
            name: passkeyName || passkeyId
        });
        setDeleteDialogOpen(true);
    };

    const confirmDeletePasskey = async () => {
        if (!passkeyToDelete) return;

        try {
            setDeletingId(passkeyToDelete.id);
            const response = await api.Client.authApi.apiV1WebauthnPasskeyIdDelete(passkeyToDelete.id);

            if (response.status === 204) {
                toast.success("Passkey deleted successfully");
            }

            // Reload passkeys list
            await loadPasskeys();
        } catch (err) {
            const e = err as { response?: { data?: { error?: string } } };
            console.error('Failed to delete passkey:', e);
            toast.error(e.response?.data?.error || "Failed to delete passkey");
        } finally {
            setDeletingId(null);
            setDeleteDialogOpen(false);
            setPasskeyToDelete(null);
        }
    };

    const formatDate = (dateString: string) => {
        return new Date(dateString).toLocaleDateString();
    };

    if (!webAuthnSupported) {
        return (
            <Card className="w-full border-none">
                <CardContent>
                    <div className="flex flex-col items-start gap-6 w-full">
                        <div className="flex items-center gap-2 w-full">
                            <div className="flex flex-col items-start gap-2">
                                <div className="[font-family:'Roboto_Mono-Bold',Helvetica] font-bold text-[var(--tailwind-colors-rdns-600)] text-base tracking-[0] leading-4">
                                    PASSKEYS
                                </div>
                                <div className="font-text-sm-leading-5-normal font-[number:var(--text-sm-leading-5-normal-font-weight)] text-[var(--tailwind-colors-slate-200)] text-[length:var(--text-sm-leading-5-normal-font-size)] tracking-[var(--text-sm-leading-5-normal-letter-spacing)] leading-[var(--text-sm-leading-5-normal-line-height)] [font-style:var(--text-sm-leading-5-normal-font-style)]">
                                    Manage your passkeys for secure passwordless authentication.
                                </div>
                            </div>
                        </div>
                        <div className="flex items-center gap-2 p-4 bg-[var(--tailwind-colors-slate-800)] border border-[var(--tailwind-colors-red-600)] rounded-lg">
                            <AlertCircle className="h-5 w-5 text-[var(--tailwind-colors-red-400)]" />
                            <div>
                                <p className="text-sm font-medium text-[var(--tailwind-colors-slate-50)]">
                                    Passkeys not supported
                                </p>
                                <p className="text-xs text-[var(--tailwind-colors-slate-200)]">
                                    Your browser doesn't support passkeys. Please use a modern browser with WebAuthn support.
                                </p>
                            </div>
                        </div>
                    </div>
                </CardContent>
            </Card>
        );
    }

    return (
        <Card className="w-full border-none">
            <CardContent>
                <div className="flex flex-col gap-6 w-full">
                    <div className="flex items-center gap-2 w-full">
                        <div className="flex flex-col items-start gap-2">
                            <div className="[font-family:'Roboto_Mono-Bold',Helvetica] font-bold text-[var(--tailwind-colors-rdns-600)] text-base tracking-[0] leading-4">
                                PASSKEYS
                            </div>
                            <div className="font-text-sm-leading-5-normal font-[number:var(--text-sm-leading-5-normal-font-weight)] text-[var(--tailwind-colors-slate-200)] text-[length:var(--text-sm-leading-5-normal-font-size)] tracking-[var(--text-sm-leading-5-normal-letter-spacing)] leading-[var(--text-sm-leading-5-normal-line-height)] [font-style:var(--text-sm-leading-5-normal-font-style)]">
                                Manage your passkeys for secure passwordless authentication.
                            </div>
                        </div>
                    </div>
                    <div className="flex flex-col gap-5">
                        <div className="space-y-4">
                            {/* Add new passkey section */}
                            <Button
                                onClick={handleAddPasskey}
                                disabled={registering}
                                className="bg-[var(--tailwind-colors-rdns-600)] hover:bg-[var(--tailwind-colors-rdns-600)]/90 w-full sm:w-auto"
                            >
                                <Plus className="h-4 w-4 mr-2" />
                                {registering ? "Adding..." : "Add Passkey"}
                            </Button>

                            {/* Existing passkeys list */}
                            {loading ? (
                                <div className="text-center py-4">
                                    <p className="text-sm text-[var(--tailwind-colors-slate-200)]">
                                        Loading passkeys...
                                    </p>
                                </div>
                            ) : passkeys.length === 0 ? (
                                <div className="flex flex-col items-center justify-center py-3 w-full">
                                    <Key className="h-8 w-8 mx-auto mb-2 text-[var(--tailwind-colors-slate-400)]" />
                                    <p className="text-sm text-[var(--tailwind-colors-slate-200)]">
                                        No passkeys registered yet
                                    </p>
                                </div>
                            ) : (
                                <div className="space-y-3">
                                    {Array.isArray(passkeys) && passkeys.map((passkey, index) => (
                                        <div
                                            key={passkey.id || index}
                                            className="flex flex-col sm:flex-row sm:items-center sm:justify-between p-4 border border-[var(--tailwind-colors-slate-600)] rounded-lg w-full gap-3 break-words"
                                        >
                                            <div className="flex items-center gap-3 w-full sm:w-auto">
                                                <div className="h-8 w-8 bg-[var(--tailwind-colors-rdns-600)] rounded-full flex items-center justify-center flex-shrink-0">
                                                    <Key className="h-4 w-4 text-[var(--tailwind-colors-slate-50)]" />
                                                </div>
                                                <div>
                                                    <p className="font-medium text-[var(--tailwind-colors-slate-50)] break-all">
                                                        {passkey.id || `Passkey ${index + 1}`}
                                                    </p>
                                                    {passkey.created_at && (
                                                        <div className="flex items-center gap-2 text-xs text-[var(--tailwind-colors-slate-200)]">
                                                            <span>Created: {formatDate(passkey.created_at)}</span>
                                                        </div>
                                                    )}
                                                </div>
                                            </div>

                                            <Button
                                                onClick={() => passkey.id && handleDeletePasskey(passkey.id, passkey.id)}
                                                disabled={deletingId === passkey.id || !passkey.id}
                                                className="h-auto min-h-11 lg:min-h-0 flex items-center justify-center px-2 py-1.5 bg-[var(--tailwind-colors-red-600)] rounded-[var(--primitives-radius-radius-md)] gap-1 hover:bg-[var(--tailwind-colors-red-400)] w-full sm:w-auto"
                                            >
                                                <Trash2 className="h-4 w-4 text-white" />
                                                <span className="text-sm text-white">Delete</span>
                                            </Button>
                                        </div>
                                    ))}
                                </div>
                            )}
                        </div>
                    </div>
                </div>
            </CardContent>

            <DeletePasskeyDialog
                open={deleteDialogOpen}
                onOpenChange={setDeleteDialogOpen}
                onConfirm={confirmDeletePasskey}
                loading={deletingId === passkeyToDelete?.id}
                passkeyName={passkeyToDelete?.name}
            />
        </Card>
    );
}

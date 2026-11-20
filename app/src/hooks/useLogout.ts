import { useContext, useState } from "react";
import { toast } from "sonner";
import { AuthContext } from "@/App";
import api from "@/api/api";

// Shared by every logout button: app state is cleared only once the API has ended the session.
// Resolves to false when the session is still active, so the caller can keep its dialog open.
export function useLogout() {
    const auth = useContext(AuthContext);
    const [loading, setLoading] = useState(false);

    const logout = async (): Promise<boolean> => {
        setLoading(true);
        try {
            await api.Client.authApi.apiV1AccountsLogoutPost();
            auth?.logout?.();
            return true;
        } catch (e) {
            // 401: the session had already ended on the server, so only app state is left to clear.
            const status = (e as { response?: { status?: number } })?.response?.status;
            if (status === 401) {
                auth?.logout?.();
                return true;
            }
            toast.error("Logout failed.");
            return false;
        } finally {
            setLoading(false);
        }
    };

    return { logout, loading };
}

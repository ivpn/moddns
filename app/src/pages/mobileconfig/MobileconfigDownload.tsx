import { useState, useEffect } from "react";
import { useParams } from "react-router-dom";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Download, AlertCircle, CheckCircle, Loader2 } from "lucide-react";
import api from "@/api/api";
import { toast } from "sonner";
import type { JSX } from "react";

export default function MobileconfigDownload(): JSX.Element {
    const { code } = useParams<{ code: string }>();
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState(false);
    const [success, setSuccess] = useState(false);
    const [errorMessage, setErrorMessage] = useState('Unable to download the configuration profile. The link may have expired or is invalid.');

    const isIOSDevice = () => {
        if (typeof navigator === 'undefined') return false;

        // iPadOS sometimes reports as MacIntel; include touch-point heuristic.
        const ua = navigator.userAgent || '';
        const isIPhoneIPadIPod = /iPad|iPhone|iPod/.test(ua);
        const isIPadOS = navigator.platform === 'MacIntel' && (navigator as unknown as { maxTouchPoints?: number }).maxTouchPoints && (navigator as unknown as { maxTouchPoints?: number }).maxTouchPoints! > 1;
        return isIPhoneIPadIPod || isIPadOS;
    };

    const downloadMobileConfig = async () => {
        if (!code) {
            setError(true);
            setErrorMessage('Invalid download link. The code parameter is missing.');
            setLoading(false);
            return;
        }

        // For iOS Safari, the configuration profile must be delivered via direct navigation
        // (not XHR/blob) to trigger the native profile download/install dialogs.
        if (isIOSDevice()) {
            const directUrl = `${import.meta.env.VITE_API_URL}/api/v1/short/${code}`;
            setLoading(false);
            setError(false);
            setSuccess(true);
            toast.success("Download started.");

            // Give React a moment to paint the success state before triggering navigation.
            window.setTimeout(() => {
                window.location.assign(directUrl);
            }, 100);
            return;
        }

        try {
            setLoading(true);
            setError(false);

            // Call the API to get the mobileconfig file using the short code
            const response = await api.Client.appleMobileconfigApi.apiV1ShortCodeGet(code, {
                responseType: 'blob'
            });

            // Create a blob URL and trigger the download
            const blob = new Blob([response.data], {
                type: 'application/x-apple-aspen-config'
            });

            // Extract filename from Content-Disposition header
            const contentDisposition = response.headers['content-disposition'];
            let filename = 'modDNS-profile.mobileconfig'; // Default fallback

            if (contentDisposition) {
                const filenameMatch = contentDisposition.match(/filename=([^;]+)/);
                if (filenameMatch && filenameMatch[1]) {
                    filename = filenameMatch[1].trim().replace(/['"]/g, '');
                }
            }

            const url = window.URL.createObjectURL(blob);
            const link = document.createElement('a');
            link.href = url;
            link.setAttribute('download', filename);
            document.body.appendChild(link);
            link.click();
            link.remove();

            // Cleanup
            window.URL.revokeObjectURL(url);

            setLoading(false);
            setSuccess(true);
            toast.success("Configuration profile downloaded successfully.");
        } catch (err) {
            console.error('Failed to download mobileconfig:', err);
            setLoading(false);
            setError(true);

            // Set a more specific error message if available
            if (err && typeof err === 'object' && 'response' in err) {
                const axiosError = err as { response?: { status?: number } };
                if (axiosError.response?.status === 404) {
                    setErrorMessage('Configuration profile not found. The link may have expired or is invalid.');
                } else if (axiosError.response?.status === 410) {
                    setErrorMessage('This download link has expired. Please generate a new configuration profile.');
                } else {
                    setErrorMessage('Failed to download the configuration profile. Please try again or generate a new link.');
                }
            }

            toast.error("Failed to download configuration profile");
        }
    };

    const retryDownload = () => {
        downloadMobileConfig();
    };

    // Start the download when the component is mounted
    useEffect(() => {
        downloadMobileConfig();
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [code]);

    return (
        <div className="fixed inset-0 flex items-center justify-center bg-[var(--shadcn-ui-app-background)] p-4">
            <div className="w-full max-w-md">
                <Card className="w-full bg-[var(--shadcn-ui-app-card)] border-[var(--shadcn-ui-app-border)]">
                    <CardContent className="p-8">
                        {loading && (
                            <div className="flex flex-col items-center justify-center py-8">
                                <Loader2 className="mb-4 h-12 w-12 animate-spin text-[var(--tailwind-colors-rdns-600)]" />
                                <h2 className="text-xl font-medium text-[var(--shadcn-ui-app-foreground)]">
                                    Loading configuration profile...
                                </h2>
                                <p className="mt-2 text-center text-[var(--shadcn-ui-app-muted-foreground)]">
                                    Your configuration profile is being prepared. It will download automatically.
                                </p>
                            </div>
                        )}

                        {error && (
                            <div className="flex flex-col items-center justify-center py-8">
                                <AlertCircle className="mb-4 h-12 w-12 text-[var(--tailwind-colors-red-600)]" />
                                <h2 className="text-xl font-medium text-[var(--tailwind-colors-red-600)]">Download Failed</h2>
                                <p className="mt-2 text-center text-[var(--shadcn-ui-app-muted-foreground)]">
                                    {errorMessage}
                                </p>
                                <Button
                                    onClick={retryDownload}
                                    className="mt-6 bg-[var(--tailwind-colors-rdns-600)] text-[var(--tailwind-colors-slate-900)] hover:bg-[var(--tailwind-colors-slate-900)] hover:text-[var(--tailwind-colors-rdns-600)]"
                                >
                                    <Download className="mr-2 h-4 w-4" />
                                    Try Again
                                </Button>
                            </div>
                        )}

                        {success && (
                            <div className="flex flex-col items-center justify-center py-8">
                                <CheckCircle className="mb-4 h-12 w-12 text-[var(--tailwind-colors-rdns-600)]" />
                                <h2 className="text-xl font-medium text-[var(--shadcn-ui-app-foreground)]">
                                    Download Started
                                </h2>
                                <p className="mt-2 text-center text-[var(--shadcn-ui-app-muted-foreground)]">
                                    Your configuration profile should be downloading now. Follow the installation prompts on your device.
                                </p>
                                <p className="mt-4 text-center text-sm text-[var(--shadcn-ui-app-muted-foreground)]">
                                    If the download didn't start automatically, click the button below.
                                </p>
                                <Button
                                    onClick={retryDownload}
                                    className="mt-6 bg-[var(--tailwind-colors-rdns-600)] hover:text-[var(--tailwind-colors-slate-50)] hover:bg-[var(--tailwind-colors-rdns-700)]"
                                >
                                    <Download className="mr-2 h-4 w-4" />
                                    Download Again
                                </Button>
                            </div>
                        )}
                    </CardContent>
                </Card>
            </div>
        </div>
    );
}

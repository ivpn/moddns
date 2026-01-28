import { useRouteError, isRouteErrorResponse, useNavigate } from "react-router-dom";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Home, RefreshCw } from "lucide-react";

export function RouterErrorBoundary() {
    const error = useRouteError();
    const navigate = useNavigate();

    let errorMessage: string;
    let errorStatus: number | undefined;

    if (isRouteErrorResponse(error)) {
        errorMessage = error.data?.message || error.statusText || "An error occurred";
        errorStatus = error.status;
    } else if (error instanceof Error) {
        errorMessage = error.message;
    } else if (typeof error === "string") {
        errorMessage = error;
    } else {
        errorMessage = "An unexpected error occurred";
    }

    const handleRetry = () => {
        window.location.reload();
    };

    const handleGoHome = () => {
        navigate("/setup");
    };

    const handleGoLogin = () => {
        navigate("/login");
    };

    return (
        <div className="fixed inset-0 flex items-center justify-center bg-background p-4">
            <Card className="w-full max-w-md p-8 bg-[var(--variable-collection-surface)] border-[var(--shadcn-ui-app-border)] shadow-2xl">
                <div className="flex flex-col items-center text-center space-y-6">
                    <div className="space-y-2">
                        <h2 className="text-2xl font-semibold text-[var(--tailwind-colors-slate-50)]">
                            {errorStatus === 401 ? "Authentication Required" : "Application Error"}
                        </h2>
                        <p className="text-[var(--tailwind-colors-slate-400)] text-base">
                            {errorStatus === 401
                                ? "Please log in to access this page."
                                : errorMessage
                            }
                        </p>
                    </div>

                    <div className="flex flex-col sm:flex-row gap-3 w-full">
                        {errorStatus === 401 ? (
                            <Button
                                onClick={handleGoLogin}
                                className="flex items-center gap-2 w-full bg-[var(--tailwind-colors-rdns-600)] text-[var(--tailwind-colors-slate-900)] hover:bg-[var(--tailwind-colors-rdns-800)]"
                            >
                                Go to Login
                            </Button>
                        ) : (
                            <>
                                <Button
                                    onClick={handleRetry}
                                    variant="outline"
                                    className="flex items-center gap-2 flex-1 border-[var(--tailwind-colors-slate-600)] text-[var(--tailwind-colors-slate-400)] hover:bg-[var(--tailwind-colors-slate-800)]"
                                >
                                    <RefreshCw className="w-4 h-4" />
                                    Try Again
                                </Button>
                                <Button
                                    onClick={handleGoHome}
                                    className="flex items-center gap-2 flex-1 bg-[var(--tailwind-colors-rdns-600)] text-[var(--tailwind-colors-slate-900)] hover:bg-[var(--tailwind-colors-rdns-800)]"
                                >
                                    <Home className="w-4 h-4" />
                                    Home
                                </Button>
                            </>
                        )}
                    </div>
                </div>
            </Card>
        </div>
    );
}

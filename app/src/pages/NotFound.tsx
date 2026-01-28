import { useNavigate } from "react-router-dom";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Home, ArrowLeft } from "lucide-react";

export default function NotFound() {
    const navigate = useNavigate();

    return (
        <div className="bg-background relative w-screen min-h-screen overflow-hidden flex items-center justify-center">
            <Card className="w-full max-w-md p-8 bg-[var(--variable-collection-surface)] border-[var(--shadcn-ui-app-border)] relative z-10">
                <div className="flex flex-col items-center text-center space-y-6">
                    {/* 404 Text */}
                    <div className="space-y-2">
                        <h1 className="text-6xl font-bold text-[var(--tailwind-colors-rdns-600)]">404</h1>
                        <h2 className="text-2xl font-semibold text-[var(--tailwind-colors-slate-50)] font-mono">
                            Page Not Found
                        </h2>
                        <p className="text-[var(--tailwind-colors-slate-400)] text-base">
                            The page you're looking for doesn't exist or has been moved.
                        </p>
                    </div>

                    {/* Action Buttons */}
                    <div className="flex flex-col sm:flex-row gap-3 w-full">
                        <Button
                            onClick={() => navigate(-1)}
                            variant="outline"
                            className="flex items-center gap-2 flex-1 border-[var(--tailwind-colors-slate-600)] text-[var(--tailwind-colors-slate-400)] hover:bg-[var(--tailwind-colors-slate-800)]"
                        >
                            <ArrowLeft className="w-4 h-4" />
                            Go Back
                        </Button>
                        <Button
                            onClick={() => navigate("/")}
                            className="flex items-center gap-2 flex-1 bg-[var(--tailwind-colors-rdns-600)] text-[var(--tailwind-colors-slate-900)] hover:bg-[var(--tailwind-colors-rdns-800)]"
                        >
                            <Home className="w-4 h-4" />
                            Home
                        </Button>
                    </div>
                </div>
            </Card>
        </div>
    );
}

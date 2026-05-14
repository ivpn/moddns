import { type JSX } from "react";
import ProfileManagementSection from "@/pages/settings/ProfileManagementSection";
import LimitedAccessBanner from "@/components/LimitedAccessBanner";
import BetaEndingBanner from "@/components/BetaEndingBanner";
import type { ModelProfile } from "@/api/client/api";

interface SettingsProps {
    profiles: ModelProfile[];
}

export default function FrameScreen({ profiles }: SettingsProps): JSX.Element {
    return (
        <div className="flex flex-col w-full gap-6 p-4 pt-8 sm:p-8 max-w-full overflow-x-hidden">
            <BetaEndingBanner />
            <LimitedAccessBanner />

            {/* Page Description */}
            <section className="w-full">
                <div className="flex flex-col gap-1">
                    <p className="text-[var(--tailwind-colors-slate-200)] text-base leading-6">
                        Settings on this page are applied only to this profile.
                    </p>
                </div>
            </section>

            <ProfileManagementSection profiles={profiles} />
        </div>
    );
}

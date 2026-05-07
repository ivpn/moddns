import { type JSX } from "react";
import SetupScreen from "@/pages/setup/SetupScreen";
import LimitedAccessBanner from "@/components/LimitedAccessBanner";
import type { ModelAccount, ModelProfile } from "@/api/client/api";

interface SettingsProps {
    account: ModelAccount;
    profiles: ModelProfile[];
}

export default function FrameScreen({ profiles }: SettingsProps): JSX.Element {
    return (
        // min-h-screen ensures iOS Safari gives the page an initial height so nested flex children render.
        // overflow-y-auto enables scrolling when right panel opens without collapsing content.
        <div className="flex flex-col w-full min-h-screen gap-4 bg-[var(--shadcn-ui-app-background)] overflow-x-hidden">
            <div className="px-8 pt-8">
                <LimitedAccessBanner />
            </div>
            <SetupScreen profiles={profiles} />
        </div>
    );
}

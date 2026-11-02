import { type JSX } from "react";
import MainContentSection from "@/pages/blocklists/MainContentSection";

export default function BlockListsMain(): JSX.Element {
    return (
        <main
            className="w-full bg-[var(--shadcn-ui-app-background)] pb-8"
        >
            <MainContentSection />
        </main>
    );
}

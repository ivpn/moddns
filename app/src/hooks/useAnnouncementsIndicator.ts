import { useEffect, useState } from "react";
import api from "@/api/api";
import { useAppStore } from "@/store/general";
import { type AnnouncementsAnnouncement, AnnouncementsSeverity } from "@/api/client";

export interface AnnouncementsIndicator {
    hasUnread: boolean;
    hasCriticalUnread: boolean;
}

// Pure unread computation (exported for testing). An announcement is "unread"
// if it was published after the user last opened the Announcements page. A null
// lastSeenAt (never opened) makes every published announcement unread.
export function computeAnnouncementsIndicator(
    items: AnnouncementsAnnouncement[],
    lastSeenAt: string | null
): AnnouncementsIndicator {
    const lastSeenMs = lastSeenAt ? new Date(lastSeenAt).getTime() : 0;
    const unread = items.filter(
        (a) => a.published_at != null && new Date(a.published_at).getTime() > lastSeenMs
    );
    return {
        hasUnread: unread.length > 0,
        hasCriticalUnread: unread.some((a) => a.severity === AnnouncementsSeverity.SeverityCritical),
    };
}

// Module-level cache so the desktop and mobile nav instances share a single
// request, and we don't refetch on every nav re-render. Errors resolve to an
// empty list — a failed fetch must never surface as a (false) indicator.
const CACHE_TTL_MS = 60_000;
let cache: { ts: number; promise: Promise<AnnouncementsAnnouncement[]> } | null = null;

function fetchAnnouncements(): Promise<AnnouncementsAnnouncement[]> {
    if (cache && Date.now() - cache.ts < CACHE_TTL_MS) {
        return cache.promise;
    }
    const promise = api.Client.announcementsApi
        .apiV1AnnouncementsGet()
        .then((resp) => resp.data ?? [])
        .catch(() => [] as AnnouncementsAnnouncement[]);
    cache = { ts: Date.now(), promise };
    return promise;
}

// useAnnouncementsIndicator fetches the announcements once (shared cache) and
// derives the unread indicator reactively against the persisted last-seen
// timestamp, so opening the Announcements page clears the dot immediately.
export function useAnnouncementsIndicator(): AnnouncementsIndicator {
    const lastSeenAt = useAppStore((s) => s.announcementsLastSeenAt);
    const [items, setItems] = useState<AnnouncementsAnnouncement[]>([]);

    useEffect(() => {
        let active = true;
        fetchAnnouncements().then((data) => {
            if (active) setItems(data);
        });
        return () => {
            active = false;
        };
    }, []);

    return computeAnnouncementsIndicator(items, lastSeenAt);
}

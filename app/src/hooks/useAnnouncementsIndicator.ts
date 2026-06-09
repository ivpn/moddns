import { useEffect, useState } from "react";
import api from "@/api/api";
import { useAppStore } from "@/store/general";
import { type AnnouncementsAnnouncement, AnnouncementsSeverity } from "@/api/client";

export interface AnnouncementsIndicator {
    hasUnread: boolean;
    hasCriticalUnread: boolean;
}

// On a clean browser (no stored last-seen) we have no record of what the user
// has read, so we only treat *recent* announcements as unread rather than the
// entire backlog. Without this floor every fresh browser would light the dot for
// historical announcements the API still returns (news/feature items rarely
// expire). Established users (a real last-seen) are unaffected — see below.
const CLEAN_BROWSER_UNREAD_WINDOW_MS = 48 * 60 * 60 * 1000; // 48h

// Pure unread computation (exported for testing). An announcement is "unread" if
// it was published after `lastSeenAt`. When `lastSeenAt` is null (clean browser,
// never opened the page) the comparison floor is `now - 48h`, so only the last
// 48h of announcements count — this keeps fresh browsers quiet about old content
// while still surfacing genuinely-recent items (including criticals). `now` is
// injectable so the window is deterministic in tests.
export function computeAnnouncementsIndicator(
    items: AnnouncementsAnnouncement[],
    lastSeenAt: string | null,
    now: number = Date.now()
): AnnouncementsIndicator {
    // Defensive: never assume the API/mocks returned an array — a bad shape here
    // must not crash the nav (which renders on every authenticated page).
    const list = Array.isArray(items) ? items : [];
    // Established user: unbounded "newer than last-seen" (someone away for a week
    // still sees the dot for anything published while gone). Clean browser: 48h floor.
    const lastSeenMs = lastSeenAt
        ? new Date(lastSeenAt).getTime()
        : now - CLEAN_BROWSER_UNREAD_WINDOW_MS;
    const unread = list.filter(
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
        .then((resp) => (Array.isArray(resp.data) ? resp.data : []))
        .catch(() => [] as AnnouncementsAnnouncement[]);
    cache = { ts: Date.now(), promise };
    return promise;
}

// How often a live session re-checks for newly-published announcements. The nav
// is mounted for the whole SPA session, so without this the dot would never
// update until a full reload.
const POLL_INTERVAL_MS = 5 * 60_000; // 5 min

// useAnnouncementsIndicator fetches the announcements (shared cache) and derives
// the unread indicator reactively against the persisted last-seen timestamp, so
// opening the Announcements page clears the dot immediately. It refreshes on tab
// re-focus and on a 5-minute poll so a newly-published announcement lights the
// dot during a live session; the 60s cache keeps these triggers from multiplying
// network calls (and dedupes the desktop/mobile nav instances).
export function useAnnouncementsIndicator(): AnnouncementsIndicator {
    const lastSeenAt = useAppStore((s) => s.announcementsLastSeenAt);
    const [items, setItems] = useState<AnnouncementsAnnouncement[]>([]);

    useEffect(() => {
        let active = true;
        const load = () =>
            fetchAnnouncements().then((data) => {
                if (active) setItems(data);
            });
        load(); // initial

        const interval = setInterval(load, POLL_INTERVAL_MS);
        const onVisible = () => {
            if (document.visibilityState === "visible") load();
        };
        document.addEventListener("visibilitychange", onVisible);

        return () => {
            active = false;
            clearInterval(interval);
            document.removeEventListener("visibilitychange", onVisible);
        };
    }, []);

    return computeAnnouncementsIndicator(items, lastSeenAt);
}

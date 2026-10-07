// Off / Statistics / Query logs control with staged changes and one Save.
//
// Source of truth: docs/specs/statistics-behaviour.md Sections L, T, C, D.

import React, { useCallback, useEffect, useId, useRef, useState } from "react";
import { Info } from "lucide-react";
import { toast } from "sonner";
import api from "@/api/api";
import type { ModelProfile } from "@/api/client";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Tooltip } from "@/components/ui/tooltip";
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogHeader,
    DialogTitle,
} from "@/components/ui/dialog";
import { DialogActions } from "@/components/dialogs/DialogLayout";
import { useSubscriptionGuard } from "@/hooks/useSubscriptionGuard";
import { useAppStore } from "@/store/general";
import { cn } from "@/lib/utils";
import {
    LIMITED_ACCESS_TEXT,
    LOGS_RETENTION_OPTIONS,
    LOGS_RETENTION_WORDS,
    SAVE_ERROR_TEXT,
    STATISTICS_RETENTION_WORDS,
    buildUpdates,
    fromProfile,
    transitionFor,
    type DataCollectionState,
    type Level,
    type Transition,
} from "./model";

export type DataCollectionPlacement = "settings" | "hero" | "logs";
export type InitialFocus = "level-logs" | "keep" | "domains" | "ips";

export interface DataCollectionControlProps {
    profile: ModelProfile;
    /** Settings and the gate dialog show every sub-option; the Off hero shows only "Also keep statistics". */
    subOptions?: "all" | "keep";
    /** Staged state to start from instead of the saved state (D2, D3). */
    initialPending?: Partial<DataCollectionState>;
    /** Element to focus on mount (D4). */
    initialFocus?: InitialFocus;
    /** Id of the visible heading that names the radio group. */
    labelledBy?: string;
    /** Rendered under the checkbox when `subOptions` is "keep" (e.g. the "More options in Settings" link). */
    keepFooter?: React.ReactNode;
    onSaved?: (profile: ModelProfile, transition: Transition) => void;
    className?: string;
}

const LEVELS: { level: Level; label: string }[] = [
    { level: "off", label: "Off" },
    { level: "stats", label: "Statistics" },
    { level: "logs", label: "Query logs" },
];

function levelDescription(level: Level, logsWords: string, statsWords: string): string {
    switch (level) {
        case "off":
            return "Nothing is stored about this profile's queries.";
        case "stats":
            return `Query counts per device, kept for ${statsWords}. No domains or IP addresses.`;
        case "logs":
            return `A record of each query (time, device, domain, result), kept for ${logsWords}. Client IP addresses only if you turn them on.`;
    }
}

const muted = "text-sm leading-5 text-[var(--tailwind-colors-slate-200)] break-words";
const strong = "font-bold text-base leading-5 text-[var(--tailwind-colors-slate-50)] break-words";
const focusRing =
    "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--tailwind-colors-rdns-600)] focus-visible:ring-offset-2 focus-visible:ring-offset-[var(--shadcn-ui-app-background)]";

interface PillOption<T extends string> {
    value: T;
    label: string;
    ariaLabel?: string;
}

function PillGroup<T extends string>({
    name,
    labelledBy,
    options,
    value,
    disabled,
    onChange,
    firstId,
}: {
    name: string;
    labelledBy: string;
    options: PillOption<T>[];
    value: T;
    disabled: boolean;
    onChange: (v: T) => void;
    firstId?: string;
}) {
    return (
        <div role="radiogroup" aria-labelledby={labelledBy} className="flex flex-wrap gap-1 self-start sm:self-auto">
            {options.map((o, i) => (
                <label
                    key={o.value}
                    className={cn(
                        "flex items-center justify-center min-h-11 lg:min-h-9 min-w-[52px] px-3 rounded border text-sm font-medium cursor-pointer select-none",
                        "has-[:focus-visible]:ring-2 has-[:focus-visible]:ring-[var(--tailwind-colors-rdns-600)]",
                        value === o.value
                            ? "bg-[var(--tailwind-colors-rdns-600)] border-[var(--tailwind-colors-rdns-600)] text-white"
                            : "border-[var(--tailwind-colors-slate-600)] text-[var(--tailwind-colors-slate-50)]",
                        disabled && "cursor-not-allowed opacity-60",
                    )}
                >
                    <input
                        type="radio"
                        className="sr-only"
                        name={name}
                        id={i === 0 ? firstId : undefined}
                        value={o.value}
                        checked={value === o.value}
                        disabled={disabled}
                        aria-label={o.ariaLabel}
                        onChange={() => onChange(o.value)}
                    />
                    {o.label}
                </label>
            ))}
        </div>
    );
}

function Hint({ children }: { children: React.ReactNode }) {
    return (
        <div role="note" className="flex gap-1.5 mt-1.5 text-[13px] leading-[18px] text-[var(--tailwind-colors-slate-100)]">
            <Info className="w-4 h-4 mt-px flex-none" aria-hidden />
            <span>{children}</span>
        </div>
    );
}

function sameState(a: DataCollectionState, b: DataCollectionState): boolean {
    return (
        a.level === b.level &&
        a.keep === b.keep &&
        a.domains === b.domains &&
        a.ips === b.ips &&
        a.retention === b.retention
    );
}

function Inner({
    profile,
    subOptions = "all",
    initialPending,
    initialFocus,
    labelledBy,
    keepFooter,
    onSaved,
    className,
}: DataCollectionControlProps) {
    const uid = useId();
    const { isRestricted } = useSubscriptionGuard();
    const setActiveProfile = useAppStore(s => s.setActiveProfile);
    const setProfiles = useAppStore(s => s.setProfiles);

    const saved = fromProfile(profile);
    const [pending, setPending] = useState<DataCollectionState>(() => ({ ...saved, ...initialPending }));
    const [saving, setSaving] = useState(false);
    const [confirm, setConfirm] = useState<Transition | null>(null);
    const levelRefs = useRef<Partial<Record<Level, HTMLInputElement | null>>>({});
    const [focusReq, setFocusReq] = useState(0);
    // The reset state after a stale-tab check; C20 shows until pending moves off it.
    const [stale, setStale] = useState<DataCollectionState | null>(null);

    const statsWords = STATISTICS_RETENTION_WORDS;
    const logsWords = LOGS_RETENTION_WORDS[pending.retention];
    const transition = transitionFor(saved, pending, statsWords);
    const disabled = isRestricted || saving;
    const idle = !transition || isRestricted;

    const ids = {
        keep: `${uid}-keep`,
        domains: `${uid}-domains`,
        ips: `${uid}-ips`,
        domainsLabel: `${uid}-domains-l`,
        ipsLabel: `${uid}-ips-l`,
        retLabel: `${uid}-ret-l`,
    };

    useEffect(() => {
        if (focusReq === 0) return;
        levelRefs.current[pending.level]?.focus({ preventScroll: true });
        // Only a new request moves focus; later pending edits must not.
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [focusReq]);

    useEffect(() => {
        if (!initialFocus) return;
        const id = initialFocus === "level-logs" ? levelRefs.current.logs?.id : ids[initialFocus];
        const frame = requestAnimationFrame(() => document.getElementById(id ?? "")?.focus({ preventScroll: false }));
        return () => cancelAnimationFrame(frame);
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, []);

    const focusChecked = useCallback(() => setFocusReq(n => n + 1), []);

    const discard = () => {
        setPending(saved);
        focusChecked();
    };

    const selectLevel = (level: Level) =>
        setPending(p => ({
            ...p,
            level,
            keep: level === "logs" ? (saved.level === "logs" ? saved.keep : true) : p.keep,
        }));

    const writeProfile = (updated: ModelProfile) => {
        setActiveProfile(updated);
        const { profiles } = useAppStore.getState();
        setProfiles(profiles.map(p => (p.profile_id === updated.profile_id ? updated : p)));
    };

    const commit = async (t: Transition) => {
        setSaving(true);
        try {
            const res = await api.Client.profilesApi.apiV1ProfilesIdPatch(profile.profile_id, {
                updates: buildUpdates(saved, pending),
            });
            const updated = res.data;
            writeProfile(updated);
            setPending(fromProfile(updated));
            setConfirm(null);
            toast.success(t.toast);
            onSaved?.(updated, t);
            focusChecked();
        } catch (e: unknown) {
            // A 5xx can follow a committed write, so show whatever the server holds now.
            try {
                const fresh = await api.Client.profilesApi.apiV1ProfilesIdGet(profile.profile_id);
                writeProfile(fresh.data);
                setPending(fromProfile(fresh.data));
            } catch {
                setPending(saved);
            }
            setConfirm(null);
            const detail = (e as { response?: { data?: { detail?: string } } })?.response?.data?.detail;
            toast.error(SAVE_ERROR_TEXT, detail ? { description: detail } : undefined);
            focusChecked();
        } finally {
            setSaving(false);
        }
    };

    const onSave = async () => {
        if (!transition || disabled) return;
        setSaving(true);
        let fresh: ModelProfile | null = null;
        try {
            fresh = (await api.Client.profilesApi.apiV1ProfilesIdGet(profile.profile_id)).data;
        } catch {
            // A failed re-read must not block the save.
        }
        if (fresh && !sameState(fromProfile(fresh), saved)) {
            writeProfile(fresh);
            const next = fromProfile(fresh);
            setPending(next);
            setStale(next);
            setSaving(false);
            focusChecked();
            return;
        }
        setSaving(false);
        if (transition.dialog) setConfirm(transition);
        else void commit(transition);
    };

    const cancelConfirm = () => {
        if (saving) return;
        setConfirm(null);
        setPending(saved);
    };

    const showAll = subOptions === "all";
    const domainHint = saved.level === "logs" && saved.domains && !pending.domains;
    const ipHint = saved.level === "logs" && saved.ips && !pending.ips;

    const onOff = [
        { value: "false", label: "Disable" },
        { value: "true", label: "Enable" },
    ];

    return (
        <div className={cn("flex flex-col gap-4 w-full", className)} data-testid="data-collection-control">
            {isRestricted && (
                <div
                    id={`${uid}-la`}
                    className="flex gap-2 rounded-md border border-[var(--tailwind-colors-slate-600)] p-3 text-sm text-[var(--tailwind-colors-slate-50)]"
                >
                    <Info className="w-4 h-4 mt-0.5 flex-none" aria-hidden />
                    <p>{LIMITED_ACCESS_TEXT}</p>
                </div>
            )}

            <div
                role="radiogroup"
                aria-labelledby={labelledBy}
                aria-label={labelledBy ? undefined : "Data collection"}
                aria-describedby={isRestricted ? `${uid}-la` : undefined}
                aria-busy={saving || undefined}
                className={cn("flex flex-col gap-2", saving && "opacity-70")}
            >
                {LEVELS.map(({ level, label }) => {
                    const checked = pending.level === level;
                    const descId = `${uid}-lvl-${level}-desc`;
                    const titleId = `${uid}-lvl-${level}-title`;
                    const inputId = `${uid}-lvl-${level}`;
                    return (
                        <div
                            key={level}
                            className={cn(
                                "rounded-lg border transition-colors",
                                checked
                                    ? "border-[var(--tailwind-colors-rdns-600)] bg-[var(--tailwind-colors-rdns-600)]/10"
                                    : "border-[var(--tailwind-colors-slate-600)]",
                                isRestricted && "opacity-60",
                            )}
                        >
                            <label
                                htmlFor={inputId}
                                className={cn(
                                    "flex gap-3 items-start px-4 py-3 min-h-14 cursor-pointer has-[:focus-visible]:ring-2 has-[:focus-visible]:ring-[var(--tailwind-colors-rdns-600)] rounded-lg",
                                    disabled && "cursor-not-allowed",
                                )}
                            >
                                <input
                                    ref={el => {
                                        levelRefs.current[level] = el;
                                    }}
                                    type="radio"
                                    className="sr-only"
                                    id={inputId}
                                    name={`${uid}-level`}
                                    value={level}
                                    checked={checked}
                                    disabled={disabled}
                                    aria-labelledby={titleId}
                                    aria-describedby={descId}
                                    onChange={() => selectLevel(level)}
                                />
                                <span
                                    aria-hidden
                                    className={cn(
                                        "mt-0.5 flex h-[18px] w-[18px] flex-none items-center justify-center rounded-full border-2",
                                        checked ? "border-[var(--tailwind-colors-rdns-600)]" : "border-[var(--tailwind-colors-slate-400)]",
                                    )}
                                >
                                    {checked && <span className="h-2 w-2 rounded-full bg-[var(--tailwind-colors-rdns-600)]" />}
                                </span>
                                <span className="flex flex-col gap-0.5 min-w-0">
                                    <span className={strong}>
                                        <span id={titleId}>{label}</span>
                                        {saved.level === level && transition && (
                                            <span className="ml-1.5 align-[2px] rounded-full border border-[var(--tailwind-colors-slate-600)] px-2 text-[11px] font-semibold text-[var(--tailwind-colors-slate-200)]">
                                                Current
                                            </span>
                                        )}
                                    </span>
                                    <span id={descId} className={muted}>
                                        {levelDescription(level, logsWords, statsWords)}
                                    </span>
                                </span>
                            </label>

                            {level === "logs" && checked && (
                                <div className="flex flex-col gap-5 border-t border-[var(--tailwind-colors-slate-600)] mx-3 sm:mx-4 py-4 sm:pl-[30px]">
                                    <div className="flex items-start gap-3 min-h-11">
                                        <Checkbox
                                            id={ids.keep}
                                            checked={pending.keep}
                                            disabled={disabled}
                                            aria-describedby={`${ids.keep}-desc`}
                                            onCheckedChange={v => setPending(p => ({ ...p, keep: v === true }))}
                                            className="mt-0.5"
                                        />
                                        <div className="flex flex-col gap-0.5 min-w-0">
                                            <label htmlFor={ids.keep} className={cn(strong, "text-sm cursor-pointer")}>
                                                Also keep statistics
                                            </label>
                                            <span id={`${ids.keep}-desc`} className={muted}>
                                                {`Counts per device for ${statsWords}. No domains or addresses.`}
                                            </span>
                                        </div>
                                    </div>

                                    {showAll && (
                                        <>
                                            <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-3">
                                                <div className="min-w-0">
                                                    <div id={ids.domainsLabel} className={cn(strong, "text-sm")}>
                                                        Log domains
                                                    </div>
                                                    <div className={muted}>Store the domain of each query.</div>
                                                    {domainHint && (
                                                        <Hint>
                                                            Applies to new queries. Existing logs keep their domains until they expire or you clear them.
                                                        </Hint>
                                                    )}
                                                </div>
                                                <PillGroup
                                                    name={`${uid}-domains`}
                                                    firstId={ids.domains}
                                                    labelledBy={ids.domainsLabel}
                                                    options={onOff}
                                                    value={String(pending.domains)}
                                                    disabled={disabled}
                                                    onChange={v => setPending(p => ({ ...p, domains: v === "true" }))}
                                                />
                                            </div>

                                            <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-3">
                                                <div className="min-w-0">
                                                    <div id={ids.ipsLabel} className={cn(strong, "text-sm")}>
                                                        Log client IP addresses
                                                    </div>
                                                    <div className={muted}>Store the IP address each query came from.</div>
                                                    {ipHint && (
                                                        <Hint>
                                                            Applies to new queries. Existing logs keep their IP addresses until they expire or you clear them.
                                                        </Hint>
                                                    )}
                                                </div>
                                                <PillGroup
                                                    name={`${uid}-ips`}
                                                    firstId={ids.ips}
                                                    labelledBy={ids.ipsLabel}
                                                    options={onOff}
                                                    value={String(pending.ips)}
                                                    disabled={disabled}
                                                    onChange={v => setPending(p => ({ ...p, ips: v === "true" }))}
                                                />
                                            </div>

                                            <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-3">
                                                <div className="min-w-0">
                                                    <div className="flex items-center gap-1">
                                                        <span id={ids.retLabel} className={cn(strong, "text-sm")}>
                                                            Retention period
                                                        </span>
                                                        <Tooltip
                                                            content={
                                                                <span>
                                                                    Changing the retention period switches to a new set of query logs. Logs collected under your previous setting remain preserved and become accessible again if you revert to that earlier retention period.
                                                                </span>
                                                            }
                                                            side="top"
                                                            align="start"
                                                            delay={0}
                                                            maxWidthClassName="max-w-[260px] md:max-w-[300px]"
                                                        >
                                                            <button
                                                                type="button"
                                                                aria-label="Retention period information"
                                                                data-testid="retention-info-trigger"
                                                                className={cn(
                                                                    "min-w-10 min-h-10 flex items-center justify-center rounded text-[var(--tailwind-colors-slate-300)] hover:text-[var(--tailwind-colors-slate-50)] transition-colors",
                                                                    focusRing,
                                                                )}
                                                            >
                                                                <Info size={16} strokeWidth={2} />
                                                            </button>
                                                        </Tooltip>
                                                    </div>
                                                    <div className={muted}>
                                                        Choose how long query logs are kept before being automatically deleted.
                                                    </div>
                                                </div>
                                                <PillGroup
                                                    name={`${uid}-retention`}
                                                    labelledBy={ids.retLabel}
                                                    options={LOGS_RETENTION_OPTIONS.map(o => ({
                                                        ...o,
                                                        ariaLabel: LOGS_RETENTION_WORDS[o.value],
                                                    }))}
                                                    value={pending.retention}
                                                    disabled={disabled}
                                                    onChange={v => setPending(p => ({ ...p, retention: v }))}
                                                />
                                            </div>
                                        </>
                                    )}
                                    {!showAll && keepFooter}
                                </div>
                            )}
                        </div>
                    );
                })}
            </div>

            <p aria-live="polite" className={cn(muted, "flex gap-1.5 min-h-5")} data-testid="data-collection-stale">
                {stale && stale === pending && (
                    <>
                        <Info className="w-4 h-4 mt-0.5 flex-none" aria-hidden />
                        <span>This setting was changed elsewhere. Review it and try again.</span>
                    </>
                )}
            </p>

            <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-3">
                <p aria-live="polite" className={cn(muted, "flex gap-1.5 min-h-5")}>
                    {!isRestricted && transition?.note ? (
                        <>
                            <Info className="w-4 h-4 mt-0.5 flex-none" aria-hidden />
                            <span>{transition.note}</span>
                        </>
                    ) : !isRestricted && !transition ? (
                        <span>No unsaved changes.</span>
                    ) : null}
                </p>
                <div className="flex gap-2">
                    <Button
                        type="button"
                        variant="outline"
                        className="min-h-11 lg:min-h-9"
                        disabled={saving || idle}
                        onClick={discard}
                    >
                        Discard
                    </Button>
                    <Button
                        type="button"
                        className="min-h-11 lg:min-h-9 bg-[var(--tailwind-colors-rdns-600)] text-white hover:bg-[var(--tailwind-colors-rdns-800)]"
                        disabled={saving || idle}
                        onClick={() => void onSave()}
                    >
                        {saving && !confirm ? "Saving…" : transition?.label ?? "Save"}
                    </Button>
                </div>
            </div>

            <Dialog open={!!confirm} onOpenChange={open => !open && cancelConfirm()}>
                <DialogContent
                    className="dialog-shell border-[var(--tailwind-colors-slate-600)] p-0 transition-opacity duration-200 [&_[data-slot=dialog-close]_svg]:text-[var(--tailwind-colors-rdns-600)] px-4 sm:px-0"
                    onCloseAutoFocus={e => {
                        e.preventDefault();
                        focusChecked();
                    }}
                >
                    <DialogHeader className="p-6 pb-0">
                        <DialogTitle className="text-lg tracking-[-0.45px] leading-[18px] font-semibold text-[var(--tailwind-colors-slate-50)]">
                            {confirm?.dialog?.title}
                        </DialogTitle>
                    </DialogHeader>
                    <DialogDescription className="px-6 pt-2 pb-0 text-sm tracking-[-0.35px] leading-[19.6px] text-[var(--tailwind-colors-slate-50)]">
                        {confirm?.dialog?.body}
                    </DialogDescription>
                    <DialogActions>
                        <Button
                            variant="cancel"
                            size="lg"
                            className="flex-1 min-w-32 font-medium"
                            onClick={cancelConfirm}
                            disabled={saving}
                        >
                            Cancel
                        </Button>
                        <Button
                            variant="default"
                            size="lg"
                            className="flex-1 min-w-32 bg-[var(--tailwind-colors-red-600)] text-white hover:bg-[var(--tailwind-colors-red-400)]"
                            onClick={() => confirm && void commit(confirm)}
                            disabled={saving}
                        >
                            {saving ? "Deleting…" : confirm?.dialog?.confirm}
                        </Button>
                    </DialogActions>
                </DialogContent>
            </Dialog>
        </div>
    );
}

// Keyed by profile so switching profiles drops staged changes (T26).
export function DataCollectionControl(props: DataCollectionControlProps) {
    return <Inner key={props.profile.profile_id} {...props} />;
}

export default DataCollectionControl;

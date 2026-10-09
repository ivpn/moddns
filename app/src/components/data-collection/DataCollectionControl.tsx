// Statistics and Query logs as two independent sources, with staged changes and one Save.
//
// Source of truth: docs/specs/statistics-behaviour.md Sections L, T, C, D, S.

import React, { useCallback, useEffect, useId, useState } from "react";
import { Info } from "lucide-react";
import { toast } from "sonner";
import api from "@/api/api";
import type { ModelProfile } from "@/api/client";
import { Button } from "@/components/ui/button";
import ToggleGroup from "@/components/general/ToggleGroup";
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
    STALE_TEXT,
    STATS_RETENTION_OPTIONS,
    buildUpdates,
    fromProfile,
    statsRetentionWords,
    transitionFor,
    type DataCollectionState,
    type Transition,
} from "./model";

export type InitialFocus = "stats" | "logs" | "stats-retention" | "domains" | "ips";
type Source = "stats" | "logs";

export interface DataCollectionControlProps {
    profile: ModelProfile;
    /** The Off hero shows only the Statistics card (D2); every other placement shows both. */
    sources?: "all" | "stats";
    /** Staged state to start from instead of the saved state (D2, D3, D4). */
    initialPending?: Partial<DataCollectionState>;
    /** Element to focus on mount (D4, S4). */
    initialFocus?: InitialFocus;
    /** Id of the visible heading that names the group of sources. */
    labelledBy?: string;
    /** Rendered at the end of the Statistics card when `sources` is "stats" (the "More options in Settings" link). */
    footer?: React.ReactNode;
    onSaved?: (profile: ModelProfile, transition: Transition) => void;
    className?: string;
}

const muted = "text-sm leading-5 text-[var(--tailwind-colors-slate-200)] break-words";
const strong = "font-bold text-base leading-5 text-[var(--tailwind-colors-slate-50)] break-words";
const focusRing =
    "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--tailwind-colors-rdns-600)] focus-visible:ring-offset-2 focus-visible:ring-offset-[var(--shadcn-ui-app-background)]";

interface PillOption<T extends string> {
    value: T;
    label: string;
    ariaLabel?: string;
    icon?: "check" | "octagon-x";
}

const subTitle =
    "[font-family:'Roboto_Flex-Medium',Helvetica] font-bold text-[var(--tailwind-colors-slate-50)] text-base tracking-[0] leading-4 break-words";
const subDesc = "text-sm leading-5 text-[var(--tailwind-colors-slate-200)] break-words";
const subRow = "flex flex-col sm:flex-row sm:items-center sm:justify-between w-full gap-3 sm:gap-4 max-w-full";
const subText = "flex flex-col items-start gap-2 min-w-0 max-w-full";
const subBlock = "flex flex-col gap-5 border-t border-[var(--tailwind-colors-slate-600)] mx-3 sm:mx-4 py-4 sm:pl-[30px]";

function PillGroup<T extends string>({
    labelledBy,
    options,
    value,
    disabled,
    onChange,
    firstId,
    checkedId,
    wide,
}: {
    labelledBy: string;
    options: PillOption<T>[];
    value: T;
    disabled: boolean;
    onChange: (v: T) => void;
    firstId?: string;
    /** Id for the selected option, so a deep link can focus it (S4). */
    checkedId?: string;
    wide?: boolean;
}) {
    return (
        <ToggleGroup
            options={options.map((o, i) => ({ ...o, id: o.value === value && checkedId ? checkedId : i === 0 ? firstId : undefined }))}
            value={value}
            // Radix reports "" when the selected pill is pressed again.
            onChange={v => v && onChange(v as T)}
            variant="outline"
            className={wide ? "!w-full sm:!w-auto" : "rounded p-0.5 self-start sm:self-auto"}
            itemClassName={wide ? "min-w-0 sm:min-w-[64px] flex-1 sm:flex-initial px-1 sm:px-3" : undefined}
            labelledBy={labelledBy}
            disabled={disabled}
        />
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

function SourceCard({
    id,
    title,
    description,
    checked,
    disabled,
    restricted,
    badge,
    onCheckedChange,
    children,
}: {
    id: string;
    title: string;
    description: string;
    checked: boolean;
    disabled: boolean;
    restricted: boolean;
    badge?: string;
    onCheckedChange: (checked: boolean) => void;
    children?: React.ReactNode;
}) {
    return (
        <div
            className={cn(
                "rounded-lg border transition-colors",
                checked
                    ? "border-[var(--tailwind-colors-rdns-600)] bg-[var(--tailwind-colors-rdns-600)]/10"
                    : "border-[var(--tailwind-colors-slate-600)]",
                restricted && "opacity-60",
            )}
        >
            <div className="flex gap-3 items-start px-4 py-3 min-h-14 rounded-lg has-[:focus-visible]:ring-2 has-[:focus-visible]:ring-[var(--tailwind-colors-rdns-600)]">
                <Checkbox
                    id={id}
                    checked={checked}
                    disabled={disabled}
                    aria-labelledby={`${id}-title`}
                    aria-describedby={`${id}-desc`}
                    onCheckedChange={v => onCheckedChange(v === true)}
                    className="mt-0.5"
                />
                <label htmlFor={id} className={cn("flex flex-col gap-0.5 min-w-0 cursor-pointer", disabled && "cursor-not-allowed")}>
                    <span className={strong}>
                        <span id={`${id}-title`}>{title}</span>
                        {badge && (
                            <span className="ml-1.5 align-[2px] rounded-full border border-[var(--tailwind-colors-slate-600)] px-2 text-[11px] font-semibold text-[var(--tailwind-colors-slate-200)]">
                                {badge}
                            </span>
                        )}
                    </span>
                    <span id={`${id}-desc`} className={muted}>
                        {description}
                    </span>
                </label>
            </div>
            {children}
        </div>
    );
}

function sameState(a: DataCollectionState, b: DataCollectionState): boolean {
    return (
        a.stats === b.stats &&
        a.logs === b.logs &&
        a.statsRetention === b.statsRetention &&
        a.domains === b.domains &&
        a.ips === b.ips &&
        a.retention === b.retention
    );
}

/** The source a change touched, for focus after save or cancel (T22); Statistics first. */
function touchedSource(saved: DataCollectionState, pending: DataCollectionState): Source {
    if (saved.stats !== pending.stats || saved.statsRetention !== pending.statsRetention) return "stats";
    return saved.logs !== pending.logs || pending.logs ? "logs" : "stats";
}

function badgeFor(saved: boolean, pending: boolean, changed: boolean): string | undefined {
    if (!changed) return undefined;
    if (saved !== pending) return pending ? "Turning on" : "Turning off";
    return saved ? "On" : undefined;
}

function Inner({
    profile,
    sources = "all",
    initialPending,
    initialFocus,
    labelledBy,
    footer,
    onSaved,
    className,
}: DataCollectionControlProps) {
    const uid = useId();
    const { isRestricted } = useSubscriptionGuard();
    const setActiveProfile = useAppStore(s => s.setActiveProfile);
    const setProfiles = useAppStore(s => s.setProfiles);

    const saved = fromProfile(profile);
    const [pending, setPending] = useState<DataCollectionState>(() => ({ ...saved, ...initialPending }));
    // L8: Statistics checked along with Query logs from Off; the hint shows until the user touches Statistics.
    const [autoChecked, setAutoChecked] = useState(
        () => !saved.stats && !saved.logs && !!initialPending?.logs && initialPending?.stats !== false,
    );
    const [saving, setSaving] = useState(false);
    const [confirm, setConfirm] = useState<Transition | null>(null);
    const [focusReq, setFocusReq] = useState<{ n: number; source: Source }>({ n: 0, source: "stats" });
    // The reset state after a stale-tab check; C20 shows until pending moves off it.
    const [stale, setStale] = useState<DataCollectionState | null>(null);

    const logsWords = LOGS_RETENTION_WORDS[pending.retention];
    const statsWords = statsRetentionWords(pending.statsRetention);
    const transition = transitionFor(saved, pending);
    const disabled = isRestricted || saving;
    const idle = !transition || isRestricted;
    const showLogs = sources === "all";

    const ids = {
        stats: `${uid}-stats`,
        logs: `${uid}-logs`,
        statsRet: `${uid}-stats-ret`,
        statsRetLabel: `${uid}-stats-ret-l`,
        domains: `${uid}-domains`,
        ips: `${uid}-ips`,
        domainsLabel: `${uid}-domains-l`,
        ipsLabel: `${uid}-ips-l`,
        retLabel: `${uid}-ret-l`,
    };
    const focusIds: Record<InitialFocus, string> = {
        stats: ids.stats,
        logs: ids.logs,
        "stats-retention": ids.statsRet,
        domains: ids.domains,
        ips: ids.ips,
    };

    useEffect(() => {
        if (focusReq.n === 0) return;
        document.getElementById(ids[focusReq.source])?.focus({ preventScroll: true });
        // Only a new request moves focus; later pending edits must not.
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [focusReq]);

    useEffect(() => {
        if (!initialFocus) return;
        const frame = requestAnimationFrame(() => {
            // S4: with statistics off there are no retention pills, so the checkbox takes focus.
            const el = document.getElementById(focusIds[initialFocus]) ?? (initialFocus === "stats-retention" ? document.getElementById(ids.stats) : null);
            el?.focus({ preventScroll: false });
        });
        return () => cancelAnimationFrame(frame);
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, []);

    const focusSource = useCallback((source: Source) => setFocusReq(r => ({ n: r.n + 1, source })), []);

    const discard = () => {
        focusSource(touchedSource(saved, pending));
        setPending(saved);
        setAutoChecked(false);
    };

    const toggleStats = (checked: boolean) => {
        setAutoChecked(false);
        setPending(p => ({ ...p, stats: checked }));
    };

    const toggleLogs = (checked: boolean) => {
        if (checked && !saved.stats && !saved.logs && !pending.stats) {
            setAutoChecked(true);
            setPending(p => ({ ...p, logs: true, stats: true }));
        } else if (!checked && autoChecked) {
            setAutoChecked(false);
            setPending(p => ({ ...p, logs: false, stats: saved.stats }));
        } else {
            setPending(p => ({ ...p, logs: checked }));
        }
    };

    const writeProfile = (updated: ModelProfile) => {
        setActiveProfile(updated);
        const { profiles } = useAppStore.getState();
        setProfiles(profiles.map(p => (p.profile_id === updated.profile_id ? updated : p)));
    };

    const commit = async (t: Transition) => {
        const source = touchedSource(saved, pending);
        setSaving(true);
        try {
            const res = await api.Client.profilesApi.apiV1ProfilesIdPatch(profile.profile_id, {
                updates: buildUpdates(saved, pending),
            });
            const updated = res.data;
            writeProfile(updated);
            setPending(fromProfile(updated));
            setAutoChecked(false);
            setConfirm(null);
            toast.success(t.toast);
            onSaved?.(updated, t);
            focusSource(source);
        } catch (e: unknown) {
            // A 5xx can follow a committed write, so show whatever the server holds now.
            try {
                const fresh = await api.Client.profilesApi.apiV1ProfilesIdGet(profile.profile_id);
                writeProfile(fresh.data);
                setPending(fromProfile(fresh.data));
            } catch {
                setPending(saved);
            }
            setAutoChecked(false);
            setConfirm(null);
            const detail = (e as { response?: { data?: { detail?: string } } })?.response?.data?.detail;
            toast.error(SAVE_ERROR_TEXT, detail ? { description: detail } : undefined);
            focusSource(source);
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
            const source = touchedSource(saved, pending);
            writeProfile(fresh);
            const next = fromProfile(fresh);
            setPending(next);
            setAutoChecked(false);
            setStale(next);
            setSaving(false);
            focusSource(source);
            return;
        }
        setSaving(false);
        if (transition.dialog) setConfirm(transition);
        else void commit(transition);
    };

    const cancelConfirm = () => {
        if (saving) return;
        setConfirm(null);
        focusSource(touchedSource(saved, pending));
        setPending(saved);
        setAutoChecked(false);
    };

    const domainHint = saved.logs && saved.domains && !pending.domains;
    const ipHint = saved.logs && saved.ips && !pending.ips;
    const lowerHint = transition?.retention === "lower";

    const onOff: PillOption<string>[] = [
        { value: "false", label: "Disable", icon: "octagon-x" },
        { value: "true", label: "Enable", icon: "check" },
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

            <div className={cn("flex flex-wrap items-center gap-1.5", muted)} data-testid="data-collection-status">
                {saved.stats || saved.logs ? (
                    <>
                        <span>Collecting now:</span>
                        {saved.stats && <StatusChip on>{`Statistics · ${statsRetentionWords(saved.statsRetention)}`}</StatusChip>}
                        {saved.logs && <StatusChip on>{`Query logs · ${LOGS_RETENTION_WORDS[saved.retention]}`}</StatusChip>}
                    </>
                ) : (
                    <StatusChip>Off · Nothing is stored about this profile's queries.</StatusChip>
                )}
            </div>

            <div
                role="group"
                aria-labelledby={labelledBy}
                aria-label={labelledBy ? undefined : "Data collection"}
                aria-describedby={isRestricted ? `${uid}-la` : undefined}
                aria-busy={saving || undefined}
                className={cn("flex flex-col gap-2", saving && "opacity-70")}
            >
                <SourceCard
                    id={ids.stats}
                    title="Statistics"
                    description={`Query counts per device, kept for ${statsWords}. No domains or IP addresses.`}
                    checked={pending.stats}
                    disabled={disabled}
                    restricted={isRestricted}
                    badge={badgeFor(saved.stats, pending.stats, !!transition)}
                    onCheckedChange={toggleStats}
                >
                    {(pending.stats || (!showLogs && footer)) && (
                        <div className={subBlock}>
                            {pending.stats && (
                                <div className={subRow}>
                                    <div className={subText}>
                                        <div id={ids.statsRetLabel} className={subTitle}>
                                            Retention period
                                        </div>
                                        <div className={subDesc}>How long counts are kept. Charts can go back this far.</div>
                                        {lowerHint && <Hint>{`Counts older than ${statsWords} will be deleted when you save.`}</Hint>}
                                    </div>
                                    <PillGroup
                                        checkedId={ids.statsRet}
                                        labelledBy={`${ids.stats}-title ${ids.statsRetLabel}`}
                                        options={STATS_RETENTION_OPTIONS.map(o => ({ value: o.value, label: o.label, ariaLabel: o.words }))}
                                        value={pending.statsRetention}
                                        disabled={disabled}
                                        onChange={v => setPending(p => ({ ...p, statsRetention: v }))}
                                    />
                                </div>
                            )}
                            {autoChecked && pending.stats && (
                                <Hint>Checked with query logs. Uncheck Statistics to keep query logs only.</Hint>
                            )}
                            {!showLogs && footer}
                        </div>
                    )}
                </SourceCard>

                {showLogs && (
                    <SourceCard
                        id={ids.logs}
                        title="Query logs"
                        description={`A record of each query (time, device, domain, result), kept for ${logsWords}. Client IP addresses only if you turn them on.`}
                        checked={pending.logs}
                        disabled={disabled}
                        restricted={isRestricted}
                        badge={badgeFor(saved.logs, pending.logs, !!transition)}
                        onCheckedChange={toggleLogs}
                    >
                        {pending.logs && (
                            <div className={subBlock}>
                                <div className={subRow}>
                                    <div className={subText}>
                                        <div id={ids.domainsLabel} className={subTitle}>
                                            Log domains
                                        </div>
                                        <div className={subDesc}>Store the domain of each query.</div>
                                        {domainHint && (
                                            <Hint>
                                                Applies to new queries. Existing logs keep their domains until they expire or you clear them.
                                            </Hint>
                                        )}
                                    </div>
                                    <PillGroup
                                        firstId={ids.domains}
                                        labelledBy={ids.domainsLabel}
                                        options={onOff}
                                        value={String(pending.domains)}
                                        disabled={disabled}
                                        onChange={v => setPending(p => ({ ...p, domains: v === "true" }))}
                                    />
                                </div>

                                <div className={subRow}>
                                    <div className={subText}>
                                        <div id={ids.ipsLabel} className={subTitle}>
                                            Log client IP addresses
                                        </div>
                                        <div className={subDesc}>Store the IP address each query came from.</div>
                                        {ipHint && (
                                            <Hint>
                                                Applies to new queries. Existing logs keep their IP addresses until they expire or you clear them.
                                            </Hint>
                                        )}
                                    </div>
                                    <PillGroup
                                        firstId={ids.ips}
                                        labelledBy={ids.ipsLabel}
                                        options={onOff}
                                        value={String(pending.ips)}
                                        disabled={disabled}
                                        onChange={v => setPending(p => ({ ...p, ips: v === "true" }))}
                                    />
                                </div>

                                <div className={subRow}>
                                    <div className={subText}>
                                        <div className="flex items-center gap-1">
                                            <span id={ids.retLabel} className={subTitle}>
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
                                        <div className={subDesc}>
                                            Choose how long query logs are kept before being automatically deleted.
                                        </div>
                                    </div>
                                    <div className="w-full sm:w-auto md:flex-shrink-0">
                                        <PillGroup
                                            wide
                                            labelledBy={`${ids.logs}-title ${ids.retLabel}`}
                                            options={LOGS_RETENTION_OPTIONS.map(o => ({
                                                ...o,
                                                ariaLabel: LOGS_RETENTION_WORDS[o.value],
                                            }))}
                                            value={pending.retention}
                                            disabled={disabled}
                                            onChange={v => setPending(p => ({ ...p, retention: v }))}
                                        />
                                    </div>
                                </div>
                            </div>
                        )}
                    </SourceCard>
                )}
            </div>

            <p aria-live="polite" className={cn(muted, "flex gap-1.5 min-h-5")} data-testid="data-collection-stale">
                {stale && stale === pending && (
                    <>
                        <Info className="w-4 h-4 mt-0.5 flex-none" aria-hidden />
                        <span>{STALE_TEXT}</span>
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
                    onCloseAutoFocus={e => e.preventDefault()}
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

function StatusChip({ on, children }: { on?: boolean; children: React.ReactNode }) {
    return (
        <span className="inline-flex items-center gap-1.5 rounded-full border border-[var(--tailwind-colors-slate-600)] px-2.5 py-0.5 text-[13px] leading-[18px] text-[var(--tailwind-colors-slate-50)] tabular-nums">
            <span
                aria-hidden
                className={cn(
                    "h-1.5 w-1.5 rounded-full",
                    on ? "bg-[var(--tailwind-colors-rdns-600)]" : "bg-[var(--tailwind-colors-slate-400)]",
                )}
            />
            {children}
        </span>
    );
}

// Keyed by profile so switching profiles drops staged changes (T26).
export function DataCollectionControl(props: DataCollectionControlProps) {
    return <Inner key={props.profile.profile_id} {...props} />;
}

export default DataCollectionControl;

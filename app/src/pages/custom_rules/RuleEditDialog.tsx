import { useEffect, useState, type JSX } from "react";
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogFooter,
    DialogHeader,
    DialogTitle,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from "@/components/ui/select";
import type { ModelCustomRule, RequestsUpdateProfileCustomRuleBody } from "@/api/client/api";

const NOTE_MAX = 80;
// Sentinel used by the group <Select>; an empty value cannot be a SelectItem.
const NO_GROUP = "__none__";

interface RuleEditDialogProps {
    rule: ModelCustomRule | null;
    open: boolean;
    onOpenChange: (open: boolean) => void;
    // Existing group names across the profile, offered as quick picks.
    existingGroups: string[];
    loading: boolean;
    onSave: (ruleId: string, patch: RequestsUpdateProfileCustomRuleBody) => void | Promise<void>;
}

export default function RuleEditDialog({
    rule,
    open,
    onOpenChange,
    existingGroups,
    loading,
    onSave,
}: RuleEditDialogProps): JSX.Element {
    const [value, setValue] = useState("");
    const [action, setAction] = useState<"block" | "allow">("block");
    const [group, setGroup] = useState("");
    const [note, setNote] = useState("");

    // Reset the form whenever a different rule is opened for editing.
    useEffect(() => {
        if (rule) {
            setValue(rule.value ?? "");
            setAction(rule.action === "allow" ? "allow" : "block");
            setGroup(rule.group ?? "");
            setNote(rule.note ?? "");
        }
    }, [rule]);

    const trimmedValue = value.trim();
    const canSave = !!rule && trimmedValue.length > 0 && !loading;

    const handleSave = () => {
        if (!rule || !canSave) return;
        // Send only changed fields so the partial-update PATCH leaves the rest intact.
        const patch: RequestsUpdateProfileCustomRuleBody = {};
        if (trimmedValue !== rule.value) patch.value = trimmedValue;
        if (action !== rule.action) patch.action = action;
        if (group !== (rule.group ?? "")) patch.group = group;
        if (note !== (rule.note ?? "")) patch.note = note;
        void onSave(rule.id, patch);
    };

    return (
        <Dialog open={open} onOpenChange={onOpenChange}>
            <DialogContent className="sm:max-w-md">
                <DialogHeader>
                    <DialogTitle>Edit rule</DialogTitle>
                    <DialogDescription>
                        Change the value, list, group, or note. Subdomain handling follows your profile setting.
                    </DialogDescription>
                </DialogHeader>

                <div className="flex flex-col gap-4 py-2">
                    <div className="flex flex-col gap-1.5">
                        <Label htmlFor="rule-edit-value">Value</Label>
                        <Input
                            id="rule-edit-value"
                            value={value}
                            onChange={(e) => setValue(e.target.value)}
                            placeholder="Domain, IP, or ASN"
                            autoComplete="off"
                            spellCheck={false}
                        />
                    </div>

                    <div className="flex flex-col gap-1.5">
                        <Label htmlFor="rule-edit-action">List</Label>
                        <Select value={action} onValueChange={(v) => setAction(v as "block" | "allow")}>
                            <SelectTrigger id="rule-edit-action">
                                <SelectValue />
                            </SelectTrigger>
                            <SelectContent>
                                <SelectItem value="block">Denylist (block)</SelectItem>
                                <SelectItem value="allow">Allowlist (allow)</SelectItem>
                            </SelectContent>
                        </Select>
                    </div>

                    <div className="flex flex-col gap-1.5">
                        <Label htmlFor="rule-edit-group">Group</Label>
                        {existingGroups.length > 0 ? (
                            <Select
                                value={group === "" ? NO_GROUP : group}
                                onValueChange={(v) => setGroup(v === NO_GROUP ? "" : v)}
                            >
                                <SelectTrigger id="rule-edit-group">
                                    <SelectValue placeholder="No group" />
                                </SelectTrigger>
                                <SelectContent>
                                    <SelectItem value={NO_GROUP}>No group</SelectItem>
                                    {existingGroups.map((g) => (
                                        <SelectItem key={g} value={g}>{g}</SelectItem>
                                    ))}
                                </SelectContent>
                            </Select>
                        ) : (
                            <p className="text-xs text-[var(--tailwind-colors-slate-400)]">
                                No groups yet — create one with “New group” on the list, then drag rules in.
                            </p>
                        )}
                    </div>

                    <div className="flex flex-col gap-1.5">
                        <Label htmlFor="rule-edit-note">Note</Label>
                        <Textarea
                            id="rule-edit-note"
                            value={note}
                            onChange={(e) => setNote(e.target.value.slice(0, NOTE_MAX))}
                            placeholder="Why this rule exists (optional)"
                            rows={3}
                            maxLength={NOTE_MAX}
                        />
                        <span className="text-xs text-[var(--tailwind-colors-slate-400)] self-end">
                            {note.length}/{NOTE_MAX}
                        </span>
                    </div>
                </div>

                <DialogFooter>
                    <Button
                        variant="cancel"
                        size="lg"
                        className="flex-1 min-w-32 font-medium"
                        onClick={() => onOpenChange(false)}
                        disabled={loading}
                    >
                        Cancel
                    </Button>
                    <Button
                        size="lg"
                        className="flex-1 min-w-32 bg-[var(--tailwind-colors-rdns-600)] text-[var(--tailwind-colors-slate-900)]"
                        onClick={handleSave}
                        disabled={!canSave}
                    >
                        Save changes
                    </Button>
                </DialogFooter>
            </DialogContent>
        </Dialog>
    );
}

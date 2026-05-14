import { Checkbox } from "@/components/ui/checkbox";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";

export type Profile = { id: string; name: string };

interface MultiSelectProfileListProps {
  profiles: Profile[];
  selectedIds: string[];
  onChange: (selectedIds: string[]) => void;
  emptyText?: string;
}

export function MultiSelectProfileList({
  profiles,
  selectedIds,
  onChange,
  emptyText = "No profiles available",
}: MultiSelectProfileListProps) {
  const total = profiles.length;
  const selectedCount = selectedIds.length;
  const allSelected = total > 0 && selectedCount === total;

  function handleSelectAll() {
    if (allSelected) {
      onChange([]);
    } else {
      onChange(profiles.map((p) => p.id));
    }
  }

  function handleToggle(id: string, checked: boolean) {
    if (checked) {
      onChange([...selectedIds, id]);
    } else {
      onChange(selectedIds.filter((sid) => sid !== id));
    }
  }

  if (total === 0) {
    return (
      <p className="text-sm text-[var(--tailwind-colors-slate-400)] py-2">
        {emptyText}
      </p>
    );
  }

  return (
    <div className="flex flex-col gap-2">
      <div className="flex items-center justify-between gap-2">
        <Button
          type="button"
          variant="ghost"
          size="sm"
          onClick={handleSelectAll}
          className="h-auto p-0 text-sm text-[var(--tailwind-colors-rdns-600)] hover:bg-transparent hover:text-[var(--tailwind-colors-rdns-600)] hover:underline"
        >
          {allSelected ? "Deselect all" : "Select all"}
        </Button>
        <Badge
          variant="outline"
          className="text-[var(--tailwind-colors-slate-200)] border-[var(--tailwind-colors-slate-600)]"
        >
          {selectedCount} / {total} selected
        </Badge>
      </div>

      <div className="flex flex-col divide-y divide-[var(--tailwind-colors-slate-700)]">
        {profiles.map((profile) => {
          const checked = selectedIds.includes(profile.id);
          return (
            <label
              key={profile.id}
              className="flex items-center gap-3 py-2.5 cursor-pointer hover:bg-[var(--tailwind-colors-slate-700)]/30 rounded px-1 transition-colors"
            >
              <Checkbox
                checked={checked}
                onCheckedChange={(value) =>
                  handleToggle(profile.id, value === true)
                }
                aria-label={`Select profile ${profile.name}`}
              />
              <span className="text-sm text-[var(--tailwind-colors-slate-50)] select-none flex-1 min-w-0 truncate">
                {profile.name}
              </span>
            </label>
          );
        })}
      </div>
    </div>
  );
}

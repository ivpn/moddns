import { useLocation, useNavigate } from "react-router-dom";
import { GlobeIcon, ShieldIcon, ListIcon, FilterX, Menu } from "lucide-react";
import { cn } from "@/lib/utils";

interface BottomNavProps {
  onMoreClick: () => void;
}

const navItems = [
  { icon: GlobeIcon, label: "Setup", path: "/setup" },
  { icon: ShieldIcon, label: "Blocklists", path: "/blocklists" },
  { icon: FilterX, label: "Rules", path: "/custom-rules" },
  { icon: ListIcon, label: "Logs", path: "/query-logs" },
];

export default function BottomNav({ onMoreClick }: BottomNavProps) {
  const location = useLocation();
  const navigate = useNavigate();

  const isActive = (path: string) =>
    location.pathname === path || location.pathname.startsWith(path + "/");

  return (
    <nav
      data-testid="bottom-nav"
      className="fixed bottom-0 left-0 right-0 z-50 flex items-stretch border-t border-border bg-[var(--sidebar-background)] pb-[env(safe-area-inset-bottom)]"
    >
      {navItems.map(({ icon: Icon, label, path }) => (
        <button
          key={path}
          onClick={() => navigate(path)}
          className={cn(
            "flex flex-col items-center justify-center flex-1 min-h-[56px] gap-0.5 text-xs transition-colors",
            isActive(path)
              ? "text-[var(--tailwind-colors-rdns-600)]"
              : "text-muted-foreground"
          )}
        >
          <Icon className="h-5 w-5" />
          <span>{label}</span>
        </button>
      ))}
      <button
        onClick={onMoreClick}
        className="flex flex-col items-center justify-center flex-1 min-h-[56px] gap-0.5 text-xs text-muted-foreground transition-colors"
      >
        <Menu className="h-5 w-5" />
        <span>More</span>
      </button>
    </nav>
  );
}

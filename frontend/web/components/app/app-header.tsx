import Link from "next/link";
import { CalendarDays, Compass, Plus, Sparkles, Wallet } from "lucide-react";

import { ProfileButton } from "@/components/app/profile-button";
import { ThemeToggle } from "@/components/app/theme-toggle";

const navigation = [
  { href: "/", label: "Discover", icon: Compass },
  { href: "/events", label: "Events", icon: CalendarDays },
  { href: "/calendars", label: "Calendars", icon: CalendarDays },
  { href: "/wallet", label: "Wallet", icon: Wallet },
  { href: "/events/create", label: "Create event", icon: Plus },
];

export function AppHeader() {
  return (
    <header className="event-create-header sticky top-0 z-20 border-b border-border/80 bg-background/90 backdrop-blur-xl">
      <div className="mx-auto flex min-h-[53px] max-w-[1240px] items-center gap-2 px-3 sm:gap-4 sm:px-6 lg:px-8">
        <Link href="/" className="flex min-h-11 min-w-11 shrink-0 items-center gap-2" aria-label="NIMNear home page">
          <span className="grid size-8 place-items-center rounded-lg bg-primary text-white"><Sparkles size={16} strokeWidth={2.2} /></span>
          <span className="hidden text-[15px] font-semibold tracking-[-0.02em] sm:inline">NIMNear</span>
        </Link>

        <nav aria-label="Main navigation" className="flex min-w-0 flex-1 items-center gap-0.5 overflow-x-auto pb-0.5">
          {navigation.map(({ href, label, icon: Icon }) => (
            <Link
              key={href}
              href={href}
              aria-label={label}
              className="inline-flex min-h-11 shrink-0 items-center gap-2 rounded-lg px-2.5 text-[13px] font-medium text-muted transition-colors hover:bg-surface-hover hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring sm:px-3"
            >
              <Icon size={15} aria-hidden="true" />
              <span className="max-[380px]:sr-only">{label}</span>
            </Link>
          ))}
        </nav>

        <div className="flex shrink-0 items-center gap-1">
          <ThemeToggle />
          <ProfileButton />
        </div>
      </div>
    </header>
  );
}

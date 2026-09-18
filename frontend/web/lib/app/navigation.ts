import { CalendarDays, Plus, type LucideIcon } from "lucide-react";

export type AppNavItem = {
  href: string;
  label: string;
  icon: LucideIcon;
};

export const appNavigation: AppNavItem[] = [
  { href: "/", label: "Events", icon: CalendarDays },
  { href: "/calendars", label: "Calendars", icon: CalendarDays },
  { href: "/events/create", label: "Create", icon: Plus },
];

export function isAppNavActive(pathname: string, href: string): boolean {
  const path = pathname.split("?")[0] || "/";

  if (href === "/") {
    if (path === "/" || path === "/events" || path === "/discover") return true;
    if (path === "/events/create" || path.startsWith("/events/create/")) return false;
    return path.startsWith("/events/");
  }

  return path === href || path.startsWith(href + "/");
}

"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";

import { appNavigation, isAppNavActive } from "@/lib/app/navigation";
import { cn } from "@/lib/utils";

export function AppNav() {
  const pathname = usePathname();

  return (
    <nav aria-label="Main navigation" className="flex min-w-0 flex-1 items-center gap-0.5 overflow-x-auto pb-0.5">
      {appNavigation.map(({ href, label, icon: Icon }) => {
        const active = isAppNavActive(pathname, href);
        return (
          <Link
            key={href}
            href={href}
            aria-label={label}
            aria-current={active ? "page" : undefined}
            className={cn(
              "inline-flex min-h-11 shrink-0 items-center gap-2 rounded-lg px-2.5 text-[13px] font-medium transition-colors hover:bg-surface-hover hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring sm:px-3",
              active ? "bg-surface-hover text-foreground" : "text-muted",
            )}
          >
            <Icon size={15} aria-hidden="true" />
            <span className="max-[380px]:sr-only">{label}</span>
          </Link>
        );
      })}
    </nav>
  );
}

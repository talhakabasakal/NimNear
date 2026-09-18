import Link from "next/link";

import { AppNav } from "@/components/app/app-nav";
import { ProfileButton } from "@/components/app/profile-button";
import { ThemeToggle } from "@/components/app/theme-toggle";
import { NimiqConnect } from "@/components/auth/nimiq-connect";

export function AppHeader() {
  return (
    <header className="event-create-header sticky top-0 z-20 border-b border-border/80 bg-background/90 backdrop-blur-xl">
      <div className="mx-auto flex min-h-[53px] max-w-[1240px] items-center gap-2 px-3 sm:gap-4 sm:px-6 lg:px-8">
        <Link href="/" className="flex min-h-11 min-w-11 shrink-0 items-center gap-2" aria-label="NIMNear home page">
          {/* eslint-disable-next-line @next/next/no-img-element */}
          <img
            src="/nimnear-logo.png"
            alt=""
            width={32}
            height={32}
            className="size-8 object-contain"
          />
          <span className="hidden text-[15px] font-semibold tracking-[-0.02em] sm:inline">NIMNear</span>
        </Link>

        <AppNav />

        <div className="flex shrink-0 items-center gap-1">
          <ThemeToggle />
          <span className="hidden sm:inline-flex">
            <NimiqConnect hideWhenAuthenticated />
          </span>
          <ProfileButton />
        </div>
      </div>
    </header>
  );
}

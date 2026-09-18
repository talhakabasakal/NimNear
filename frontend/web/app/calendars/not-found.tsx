import Link from "next/link";

import { AppHeader } from "@/components/app/app-header";
import { StateCard } from "@/components/app/state-card";

export default function NotFound() {
  return <div className="min-h-svh bg-background"><AppHeader /><main className="mx-auto max-w-[760px] px-4 py-10 sm:px-6 lg:px-8"><StateCard kind="empty" title="Calendar not found" description="This calendar is not public or is no longer available." /><Link href="/calendars" className="mt-5 block text-center text-xs font-medium text-accent hover:text-foreground">Back to calendars</Link></main></div>;
}

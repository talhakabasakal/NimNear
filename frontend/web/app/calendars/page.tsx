import { ArrowLeft, CalendarDays } from "lucide-react";
import Link from "next/link";

import { AppHeader } from "@/components/app/app-header";
import { StateCard } from "@/components/app/state-card";
import { CalendarCard } from "@/components/calendars/calendar-card";
import { CalendarFollowButton } from "@/components/calendars/calendar-follow-button";
import { CalendarWorkspace } from "@/components/calendars/calendar-workspace";
import { fetchCalendars, type CalendarRecord } from "@/lib/api/calendars";

export const dynamic = "force-dynamic";

export default async function CalendarsPage() {
  let calendars: CalendarRecord[] = [];
  let error = false;
  try {
    calendars = await fetchCalendars();
  } catch {
    error = true;
  }

  return (
    <div className="min-h-svh bg-background">
      <AppHeader />
      <main className="mx-auto max-w-[1120px] space-y-10 px-4 py-8 sm:px-6 sm:py-10 lg:px-8">
        <div>
          <Link href="/" className="mb-5 inline-flex items-center gap-2 text-xs font-medium text-muted transition-colors hover:text-foreground"><ArrowLeft size={14} /> Discover</Link>
          <div className="flex items-center gap-3"><span className="grid size-10 place-items-center rounded-xl bg-primary/15 text-accent"><CalendarDays size={19} /></span><div><p className="text-xs font-medium uppercase tracking-[0.15em] text-accent">Community</p><h1 className="mt-1 text-3xl font-semibold tracking-[-0.035em] text-foreground">Calendars</h1></div></div>
          <p className="mt-3 max-w-xl text-sm leading-6 text-muted">Discover public event calendars from communities.</p>
        </div>

        <section className="rounded-xl border border-border bg-surface p-5 sm:p-6">
          <p className="text-xs font-medium uppercase tracking-[0.16em] text-accent">Explore calendars</p>
          <h2 className="mt-1 text-xl font-semibold text-foreground">Follow events by community</h2>
          <p className="mt-2 max-w-xl text-sm leading-6 text-muted">Browse public calendars, or create and follow your own with a backend session.</p>
          <Link href="#my-calendars" className="mt-4 inline-flex h-9 items-center rounded-lg bg-primary px-3 text-xs font-medium text-white transition-opacity hover:opacity-90">Next</Link>
        </section>

        <section className="space-y-4">
          <div><p className="text-xs font-medium uppercase tracking-[0.16em] text-accent">Discover</p><h2 className="mt-1 text-xl font-semibold text-foreground">Community calendars</h2></div>
          {error ? <div className="space-y-3"><StateCard kind="error" title="Calendars could not be loaded" description="The calendar service is currently unavailable." /><Link href="/calendars" className="block text-center text-xs font-medium text-accent hover:text-foreground">Try again</Link></div> : calendars.length === 0 ? <StateCard kind="empty" title="No public calendars yet" description="Community calendars will appear here once they are created." /> : <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">{calendars.map((calendar) => <div key={calendar.id} className="space-y-3"><CalendarCard calendar={calendar} /><div className="flex justify-end"><CalendarFollowButton calendarId={calendar.id} /></div></div>)}</div>}
        </section>

        <section id="my-calendars"><CalendarWorkspace /></section>
      </main>
    </div>
  );
}

import { ArrowRight } from "lucide-react";
import Link from "next/link";

import { AppHeader } from "@/components/app/app-header";
import { StateCard } from "@/components/app/state-card";
import { CalendarCard } from "@/components/calendars/calendar-card";
import { NearbyPlaceDiscovery } from "@/components/discover/nearby-place-discovery";
import { DiscoverHero } from "@/components/discover/discover-hero";
import { EventList } from "@/components/events/event-list";
import { fetchEvents, type EventRecord } from "@/lib/api/events";
import { fetchCalendars, type CalendarRecord } from "@/lib/api/calendars";

export const dynamic = "force-dynamic";

async function loadHomeEvents(): Promise<{ events: EventRecord[]; error?: string }> {
  try {
    return { events: await fetchEvents({ limit: 4 }) };
  } catch {
    return { events: [], error: "api" };
  }
}

async function loadHomeCalendars(): Promise<{ calendars: CalendarRecord[]; error?: string }> {
  try {
    return { calendars: await fetchCalendars(3) };
  } catch {
    return { calendars: [], error: "api" };
  }
}

export default async function Home() {
  const [result, calendarResult] = await Promise.all([loadHomeEvents(), loadHomeCalendars()]);

  return (
    <div className="min-h-svh bg-background">
      <AppHeader />
      <main className="mx-auto max-w-[1240px] space-y-12 px-4 py-6 sm:px-6 sm:py-8 lg:px-8">
        <DiscoverHero eventCount={result.events.length} hasError={Boolean(result.error)} />

        <section>
          <div className="mb-4 flex items-end justify-between gap-4">
            <div>
              <p className="text-xs font-medium uppercase tracking-[0.16em] text-accent">Event feed</p>
              <h2 className="mt-1 text-xl font-semibold tracking-[-0.025em] text-foreground">Upcoming events</h2>
            </div>
            <Link href="/events" className="inline-flex items-center gap-1.5 text-xs font-medium text-muted transition-colors hover:text-foreground">
              View all <ArrowRight size={14} />
            </Link>
          </div>
          <EventList events={result.events} error={result.error} compact />
        </section>

        <section>
          <div className="mb-4 flex items-end justify-between gap-4">
            <div>
              <p className="text-xs font-medium uppercase tracking-[0.16em] text-accent">Community</p>
              <h2 className="mt-1 text-xl font-semibold tracking-[-0.025em] text-foreground">Community calendars</h2>
            </div>
            <Link href="/calendars" className="inline-flex items-center gap-1.5 text-xs font-medium text-muted transition-colors hover:text-foreground">View all <ArrowRight size={14} /></Link>
          </div>
          {calendarResult.error ? <StateCard kind="error" title="Calendars could not be loaded" description="The calendar service is currently unavailable." /> : calendarResult.calendars.length === 0 ? <StateCard kind="empty" title="No public calendars yet" description="Community calendars will appear here once they are created." /> : <div className="grid gap-3 sm:grid-cols-3">{calendarResult.calendars.map((calendar) => <CalendarCard key={calendar.id} calendar={calendar} />)}</div>}
        </section>

        <NearbyPlaceDiscovery />
      </main>
    </div>
  );
}

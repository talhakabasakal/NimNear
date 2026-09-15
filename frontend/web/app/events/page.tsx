import { ArrowLeft, CalendarDays } from "lucide-react";
import Link from "next/link";

import { AppHeader } from "@/components/app/app-header";
import { EventTimeline } from "@/components/events/event-timeline";
import { fetchEvents, type EventRecord } from "@/lib/api/events";

export const dynamic = "force-dynamic";

type EventsPageProps = {
  searchParams: Promise<{ view?: string }>;
};

async function loadEvents(view: "upcoming" | "past"): Promise<{ events: EventRecord[]; error?: string }> {
  try {
    const now = new Date().toISOString();
    const events = await fetchEvents(view === "past" ? { to: now, limit: 100 } : { from: now, limit: 100 });
    return { events: view === "past" ? events.filter((event) => event.is_past) : events };
  } catch {
    return { events: [], error: "api" };
  }
}

export default async function EventsPage({ searchParams }: EventsPageProps) {
  const params = await searchParams;
  const view = params.view === "past" ? "past" : "upcoming";
  const result = await loadEvents(view);

  return (
    <div className="min-h-svh bg-background">
      <AppHeader />
      <main className="mx-auto max-w-[1120px] px-4 py-8 sm:px-6 sm:py-10 lg:px-8">
        <div className="mb-8 flex flex-wrap items-end justify-between gap-5">
          <div>
            <Link href="/" className="mb-5 inline-flex items-center gap-2 text-xs font-medium text-muted transition-colors hover:text-foreground"><ArrowLeft size={14} /> Keşfet</Link>
            <div className="flex items-center gap-3">
              <span className="grid size-10 place-items-center rounded-xl bg-primary/15 text-accent"><CalendarDays size={19} /></span>
              <div><p className="text-xs font-medium uppercase tracking-[0.15em] text-accent">İstanbul</p><h1 className="mt-1 text-3xl font-semibold tracking-[-0.035em] text-foreground">Etkinlikler</h1></div>
            </div>
          </div>
          <nav className="flex rounded-lg border border-border bg-surface p-1" aria-label="Etkinlik zamanı">
            <Link href="/events?view=upcoming" className={"rounded-md px-3 py-2 text-xs font-medium transition-colors " + (view === "upcoming" ? "bg-surface-hover text-foreground" : "text-muted hover:text-foreground")}>Yaklaşan</Link>
            <Link href="/events?view=past" className={"rounded-md px-3 py-2 text-xs font-medium transition-colors " + (view === "past" ? "bg-surface-hover text-foreground" : "text-muted hover:text-foreground")}>Geçmiş</Link>
          </nav>
        </div>
        <EventTimeline events={result.events} error={result.error} />
      </main>
    </div>
  );
}

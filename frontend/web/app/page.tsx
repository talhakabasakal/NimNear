import { ArrowRight, BookOpen, Coffee, Compass, Music2, Palette, Users } from "lucide-react";
import Link from "next/link";

import { AppHeader } from "@/components/app/app-header";
import { StateCard } from "@/components/app/state-card";
import { CategoryCard } from "@/components/discover/category-card";
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

const categories = [
  { label: "Müzik", icon: Music2, tone: "bg-[#d7a6ff]/10 text-[#d7a6ff]" },
  { label: "Sanat", icon: Palette, tone: "bg-[#8277ff]/10 text-[#a49cff]" },
  { label: "Topluluk", icon: Users, tone: "bg-[#5b5af7]/15 text-[#9b99ff]" },
  { label: "Atölye", icon: BookOpen, tone: "bg-[#f1b7a2]/10 text-[#f1b7a2]" },
  { label: "Buluşma", icon: Coffee, tone: "bg-[#d8c09a]/10 text-[#d8c09a]" },
  { label: "Keşif", icon: Compass, tone: "bg-[#9ad9e6]/10 text-[#9ad9e6]" },
];

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
              <p className="text-xs font-medium uppercase tracking-[0.16em] text-accent">Etkinlik akışı</p>
              <h2 className="mt-1 text-xl font-semibold tracking-[-0.025em] text-foreground">Yaklaşan etkinlikler</h2>
            </div>
            <Link href="/events" className="inline-flex items-center gap-1.5 text-xs font-medium text-muted transition-colors hover:text-foreground">
              Tümünü görüntüle <ArrowRight size={14} />
            </Link>
          </div>
          <EventList events={result.events} error={result.error} compact />
        </section>

        <section>
          <div className="mb-4">
            <p className="text-xs font-medium uppercase tracking-[0.16em] text-accent">İlgi alanın</p>
            <h2 className="mt-1 text-xl font-semibold tracking-[-0.025em] text-foreground">Neyi keşfetmek istersin?</h2>
          </div>
          <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-6">
            {categories.map((category) => <CategoryCard key={category.label} {...category} />)}
          </div>
        </section>

        <section>
          <div className="mb-4 flex items-end justify-between gap-4">
            <div>
              <p className="text-xs font-medium uppercase tracking-[0.16em] text-accent">Topluluk</p>
              <h2 className="mt-1 text-xl font-semibold tracking-[-0.025em] text-foreground">Topluluk takvimleri</h2>
            </div>
            <Link href="/calendars" className="inline-flex items-center gap-1.5 text-xs font-medium text-muted transition-colors hover:text-foreground">Tümünü görüntüle <ArrowRight size={14} /></Link>
          </div>
          {calendarResult.error ? <StateCard kind="error" title="Takvimler yüklenemedi" description="Takvim servisine şu anda ulaşılamıyor." /> : calendarResult.calendars.length === 0 ? <StateCard kind="empty" title="Henüz herkese açık takvim yok" description="Topluluk takvimleri oluşturulduğunda burada görünecek." /> : <div className="grid gap-3 sm:grid-cols-3">{calendarResult.calendars.map((calendar) => <CalendarCard key={calendar.id} calendar={calendar} />)}</div>}
        </section>

        <NearbyPlaceDiscovery />
      </main>
    </div>
  );
}

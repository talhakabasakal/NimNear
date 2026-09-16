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
          <Link href="/" className="mb-5 inline-flex items-center gap-2 text-xs font-medium text-muted transition-colors hover:text-foreground"><ArrowLeft size={14} /> Keşfet</Link>
          <div className="flex items-center gap-3"><span className="grid size-10 place-items-center rounded-xl bg-primary/15 text-accent"><CalendarDays size={19} /></span><div><p className="text-xs font-medium uppercase tracking-[0.15em] text-accent">Topluluk</p><h1 className="mt-1 text-3xl font-semibold tracking-[-0.035em] text-foreground">Takvimler</h1></div></div>
          <p className="mt-3 max-w-xl text-sm leading-6 text-muted">Toplulukların herkese açık etkinlik takvimlerini keşfet.</p>
        </div>

        <section className="rounded-xl border border-border bg-surface p-5 sm:p-6">
          <p className="text-xs font-medium uppercase tracking-[0.16em] text-accent">Takvimleri keşfet</p>
          <h2 className="mt-1 text-xl font-semibold text-foreground">Etkinlikleri topluluklarına göre takip et</h2>
          <p className="mt-2 max-w-xl text-sm leading-6 text-muted">Herkese açık takvimleri inceleyebilir, backend oturumun olduğunda kendi takvimlerini oluşturup takip edebilirsin.</p>
          <Link href="#my-calendars" className="mt-4 inline-flex h-9 items-center rounded-lg bg-primary px-3 text-xs font-medium text-white transition-opacity hover:opacity-90">İleri</Link>
        </section>

        <section className="space-y-4">
          <div><p className="text-xs font-medium uppercase tracking-[0.16em] text-accent">Keşfet</p><h2 className="mt-1 text-xl font-semibold text-foreground">Topluluk takvimleri</h2></div>
          {error ? <div className="space-y-3"><StateCard kind="error" title="Takvimler yüklenemedi" description="Takvim servisine şu anda ulaşılamıyor." /><Link href="/calendars" className="block text-center text-xs font-medium text-accent hover:text-foreground">Tekrar dene</Link></div> : calendars.length === 0 ? <StateCard kind="empty" title="Henüz herkese açık takvim yok" description="Topluluk takvimleri oluşturulduğunda burada görünecek." /> : <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">{calendars.map((calendar) => <div key={calendar.id} className="space-y-3"><CalendarCard calendar={calendar} /><div className="flex justify-end"><CalendarFollowButton calendarId={calendar.id} /></div></div>)}</div>}
        </section>

        <section id="my-calendars"><CalendarWorkspace /></section>
      </main>
    </div>
  );
}

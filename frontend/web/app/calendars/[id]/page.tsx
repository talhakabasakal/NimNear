import { ArrowLeft, CalendarDays } from "lucide-react";
import Link from "next/link";
import { notFound } from "next/navigation";

import { AppHeader } from "@/components/app/app-header";
import { StateCard } from "@/components/app/state-card";
import { CalendarFollowButton } from "@/components/calendars/calendar-follow-button";
import { EventTimeline } from "@/components/events/event-timeline";
import { ExternalMediaImage } from "@/components/media/external-media-image";
import { CalendarsApiError, fetchCalendar, type CalendarDetailRecord } from "@/lib/api/calendars";

export const dynamic = "force-dynamic";

type CalendarPageProps = { params: Promise<{ id: string }> };

function CalendarError() {
  return <div className="min-h-svh bg-background"><AppHeader /><main className="mx-auto max-w-[760px] px-4 py-10 sm:px-6 lg:px-8"><Link href="/calendars" className="mb-6 inline-flex items-center gap-2 text-xs font-medium text-muted hover:text-foreground"><ArrowLeft size={14} /> Takvimler</Link><StateCard kind="error" title="Takvim yüklenemedi" description="Takvim servisine şu anda ulaşılamıyor." /></main></div>;
}

function CalendarHero({ calendar }: { calendar: CalendarDetailRecord["data"] }) {
  const fallback = <div className="grid size-full place-items-center bg-primary/15 text-accent" aria-hidden="true"><CalendarDays size={30} /></div>;
  return <section className="overflow-hidden rounded-xl border border-border bg-surface"><div className="h-48 sm:h-64"><ExternalMediaImage src={calendar.image_url} alt="" className="size-full object-cover" fallback={fallback} /></div><div className="flex flex-wrap items-start justify-between gap-4 p-5 sm:p-7"><div className="space-y-2"><p className="text-xs font-medium uppercase tracking-[0.16em] text-accent">Topluluk takvimi</p><h1 className="text-3xl font-semibold tracking-[-0.035em] text-foreground sm:text-[42px]">{calendar.name}</h1>{calendar.description ? <p className="max-w-2xl text-sm leading-6 text-muted">{calendar.description}</p> : null}</div><CalendarFollowButton calendarId={calendar.id} /></div></section>;
}

export default async function CalendarPage({ params }: CalendarPageProps) {
  const { id } = await params;
  let detail: CalendarDetailRecord;
  try {
    detail = await fetchCalendar(id);
  } catch (error) {
    if (error instanceof CalendarsApiError && error.status === 404) notFound();
    return <CalendarError />;
  }

  return <div className="min-h-svh bg-background"><AppHeader /><main className="mx-auto max-w-[1120px] space-y-8 px-4 py-8 sm:px-6 sm:py-10 lg:px-8"><Link href="/calendars" className="inline-flex items-center gap-2 text-xs font-medium text-muted hover:text-foreground"><ArrowLeft size={14} /> Takvimler</Link><CalendarHero calendar={detail.data} /><section className="space-y-4"><div><p className="text-xs font-medium uppercase tracking-[0.16em] text-accent">Etkinlik akışı</p><h2 className="mt-1 text-xl font-semibold text-foreground">Bu takvimdeki etkinlikler</h2></div><EventTimeline events={detail.events} /></section></main></div>;
}

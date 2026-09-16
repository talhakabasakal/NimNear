import { ArrowLeft } from "lucide-react";
import Link from "next/link";
import { notFound } from "next/navigation";

import { AppHeader } from "@/components/app/app-header";
import { StateCard } from "@/components/app/state-card";
import { EventDetail } from "@/components/events/event-detail";
import { EventsApiError, fetchEvent, type EventRecord } from "@/lib/api/events";
import { fetchProfile, type ProfileRecord } from "@/lib/api/profiles";

export const dynamic = "force-dynamic";

type EventDetailPageProps = {
  params: Promise<{ id: string }>;
};

function BackToEvents() {
  return <Link href="/events" className="mb-6 inline-flex items-center gap-2 text-xs font-medium text-muted transition-colors hover:text-foreground"><ArrowLeft size={14} /> Etkinlikler</Link>;
}

function EventLoadError() {
  return (
    <div className="min-h-svh bg-background">
      <AppHeader />
      <main className="mx-auto max-w-[1120px] px-4 py-8 sm:px-6 sm:py-10 lg:px-8">
        <BackToEvents />
        <StateCard kind="error" title="Etkinlik yüklenemedi" description="Etkinlik servisine şu anda ulaşılamıyor." />
      </main>
    </div>
  );
}

export default async function EventDetailPage({ params }: EventDetailPageProps) {
  const { id } = await params;
  let event: EventRecord;
  let organizer: ProfileRecord | null = null;
  let organizerUnavailable = false;

  try {
    event = await fetchEvent(id);
    if (event.organizer_id) {
      try {
        organizer = await fetchProfile(event.organizer_id);
      } catch {
        organizerUnavailable = true;
      }
    }
  } catch (error) {
    if (error instanceof EventsApiError && error.status === 404) notFound();
    return <EventLoadError />;
  }

  return (
    <div className="min-h-svh bg-background">
      <AppHeader />
      <main className="mx-auto max-w-[1120px] px-4 py-8 sm:px-6 sm:py-10 lg:px-8">
        <BackToEvents />
        <EventDetail event={event} organizer={organizer} organizerUnavailable={organizerUnavailable} />
      </main>
    </div>
  );
}

import { ArrowLeft, MapPin } from "lucide-react";
import Link from "next/link";
import { notFound } from "next/navigation";

import { AppHeader } from "@/components/app/app-header";
import { StateCard } from "@/components/app/state-card";
import { EventTimeline } from "@/components/events/event-timeline";
import { ExternalMediaImage } from "@/components/media/external-media-image";
import { EventsApiError, fetchEvents, type EventRecord } from "@/lib/api/events";
import { fetchPlace, PlacesApiError, type PlaceRecord } from "@/lib/api/places";

export const dynamic = "force-dynamic";

type PlacePageProps = { params: Promise<{ id: string }> };

function PlaceError() {
  return (
    <div className="min-h-svh bg-background">
      <AppHeader />
      <main className="mx-auto max-w-[1120px] px-4 py-8 sm:px-6 sm:py-10 lg:px-8">
        <Link href="/" className="mb-6 inline-flex items-center gap-2 text-xs font-medium text-muted transition-colors hover:text-foreground"><ArrowLeft size={14} /> Keşfet</Link>
        <StateCard kind="error" title="Yer yüklenemedi" description="Yer servisine şu anda ulaşılamıyor." />
      </main>
    </div>
  );
}

function PlaceHero({ place }: { place: PlaceRecord }) {
  const fallback = <div className="grid size-full place-items-center bg-[radial-gradient(circle_at_25%_20%,#8277ff_0,transparent_32%),linear-gradient(135deg,#251e3a,#171621_60%,#382447)]" aria-hidden="true"><MapPin size={28} className="text-accent/80" /></div>;
  return (
    <section className="overflow-hidden rounded-xl border border-border bg-surface">
      <div className="h-48 sm:h-64"><ExternalMediaImage src={place.image_url || null} alt="" className="size-full object-cover" fallback={fallback} /></div>
      <div className="space-y-3 p-5 sm:p-7">
        {place.category ? <p className="text-xs font-medium uppercase tracking-[0.16em] text-accent">{place.category}</p> : null}
        <h1 className="text-3xl font-semibold tracking-[-0.035em] text-foreground sm:text-[42px]">{place.name}</h1>
        {place.description ? <p className="max-w-2xl text-sm leading-6 text-muted">{place.description}</p> : null}
        {place.address ? <p className="flex items-center gap-2 text-sm text-muted"><MapPin size={15} className="text-accent" />{place.address}</p> : null}
      </div>
    </section>
  );
}

export default async function PlacePage({ params }: PlacePageProps) {
  const { id } = await params;
  let place: PlaceRecord;
  try {
    place = await fetchPlace(id);
  } catch (error) {
    if (error instanceof PlacesApiError && error.status === 404) notFound();
    return <PlaceError />;
  }

  let events: EventRecord[] = [];
  let eventsError: string | undefined;
  try {
    events = await fetchEvents({ place_id: id, from: new Date().toISOString(), limit: 100 });
  } catch (error) {
    eventsError = error instanceof EventsApiError ? error.message : "Etkinlik servisine şu anda ulaşılamıyor.";
  }

  return (
    <div className="min-h-svh bg-background">
      <AppHeader />
      <main className="mx-auto max-w-[1120px] space-y-8 px-4 py-8 sm:px-6 sm:py-10 lg:px-8">
        <Link href="/" className="inline-flex items-center gap-2 text-xs font-medium text-muted transition-colors hover:text-foreground"><ArrowLeft size={14} /> Keşfet</Link>
        <PlaceHero place={place} />
        <section className="space-y-4">
          <div><p className="text-xs font-medium uppercase tracking-[0.16em] text-accent">Etkinlikler</p><h2 className="mt-1 text-xl font-semibold tracking-[-0.025em] text-foreground">Bu yerde yaklaşan etkinlikler</h2></div>
          <EventTimeline events={events} error={eventsError} />
        </section>
      </main>
    </div>
  );
}

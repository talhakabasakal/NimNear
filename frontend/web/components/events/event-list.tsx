import type { EventRecord } from "@/lib/api/events";
import { resolveCollectionState } from "@/lib/collection-state";

import { StateCard } from "@/components/app/state-card";

import { EventCard } from "./event-card";

type EventListProps = {
  events: EventRecord[];
  error?: string;
  compact?: boolean;
};

export function EventList({ events, error, compact = false }: EventListProps) {
  const state = resolveCollectionState(events, error);
  if (state.kind === "error") return <StateCard kind="error" title="Etkinlikler yüklenemedi" description="Bağlantıyı kontrol edip tekrar deneyebilirsin." />;
  if (state.kind === "empty") return <StateCard kind="empty" title="Henüz etkinlik yok" description="Bu bölümde görünecek yeni etkinlikler burada listelenecek." />;

  return (
    <div className={"grid gap-4 " + (compact ? "sm:grid-cols-2 lg:grid-cols-4" : "md:grid-cols-2")}>
      {state.records.map((event) => <EventCard key={event.id} event={event} compact={compact} />)}
    </div>
  );
}

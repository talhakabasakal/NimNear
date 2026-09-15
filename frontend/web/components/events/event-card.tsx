import { CalendarDays, Clock3, MapPin, Users } from "lucide-react";
import Link from "next/link";

import { formatNimPrice, type EventRecord } from "@/lib/api/events";

import { EventStatusBadge } from "./event-status-badge";

type EventCardProps = {
  event: EventRecord;
  compact?: boolean;
};

const dateFormatter = new Intl.DateTimeFormat("tr-TR", { day: "numeric", month: "short" });
const timeFormatter = new Intl.DateTimeFormat("tr-TR", { hour: "2-digit", minute: "2-digit" });


function EventArtwork({ event, compact }: EventCardProps) {
  if (event.image_url) {
    return (
      <div className={"relative overflow-hidden bg-surface-hover " + (compact ? "h-32" : "h-40")}>
        <img src={event.image_url} alt="" className="size-full object-cover" />
        <div className="absolute inset-0 bg-gradient-to-t from-[#0f0e10]/70 via-transparent to-transparent" />
      </div>
    );
  }

  return (
    <div className={"relative overflow-hidden bg-[radial-gradient(circle_at_25%_20%,#8277ff_0,transparent_32%),linear-gradient(135deg,#251e3a,#171621_60%,#382447)] " + (compact ? "h-32" : "h-40")} aria-hidden="true">
      <div className="absolute -right-8 -top-10 size-36 rounded-full border border-accent/20" />
      <div className="absolute bottom-0 left-0 h-1/2 w-full bg-gradient-to-t from-[#0f0e10]/50 to-transparent" />
    </div>
  );
}

export function EventCard({ event, compact = false }: EventCardProps) {
  return (
    <Link href={`/events/${event.id}`} className="group block rounded-xl focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring" aria-label={event.title}>
      <article className="overflow-hidden rounded-xl border border-border bg-surface transition-colors group-hover:bg-surface-hover">
      <EventArtwork event={event} compact={compact} />
      <div className="space-y-3 p-4">
        <div className="flex items-start justify-between gap-3">
          <div className="min-w-0">
            <p className="mb-1 text-[11px] font-medium uppercase tracking-[0.14em] text-accent">{event.city}</p>
            <h3 className="line-clamp-2 text-[15px] font-semibold leading-5 text-foreground">{event.title}</h3>
          </div>
          <EventStatusBadge event={event} />
        </div>

        <div className="grid gap-2 text-xs text-muted">
          <span className="flex items-center gap-2"><CalendarDays size={14} className="text-accent/80" />{dateFormatter.format(new Date(event.starts_at))}</span>
          <span className="flex items-center gap-2"><Clock3 size={14} className="text-accent/80" />{timeFormatter.format(new Date(event.starts_at))}</span>
          <span className="flex items-center gap-2"><MapPin size={14} className="text-accent/80" />{event.address || event.city}</span>
        </div>

        <div className="flex items-center justify-between border-t border-border pt-3 text-xs text-muted">
          <span className="flex items-center gap-1.5"><Users size={14} />{event.attendee_count} katılımcı</span>
          <span className="font-medium text-foreground">{event.is_free ? "Ücretsiz" : formatNimPrice(event.price_nim) + " " + event.currency}</span>
        </div>
      </div>
      </article>
    </Link>
  );
}

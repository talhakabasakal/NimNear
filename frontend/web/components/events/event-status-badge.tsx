import { formatNimPrice, type EventRecord } from "@/lib/api/events";

type EventStatusBadgeProps = {
  event: EventRecord;
};

export function EventStatusBadge({ event }: EventStatusBadgeProps) {
  if (event.is_past) return <span className="rounded-full bg-surface-hover px-2.5 py-1 text-[11px] font-medium text-muted">Geçmiş</span>;
  if (event.is_sold_out) return <span className="rounded-full bg-red-400/10 px-2.5 py-1 text-[11px] font-medium text-red-200">Tükendi</span>;
  if (event.is_free) return <span className="rounded-full bg-accent/10 px-2.5 py-1 text-[11px] font-medium text-accent">Ücretsiz</span>;
  return <span className="rounded-full bg-primary/10 px-2.5 py-1 text-[11px] font-medium text-[#b9b3ff]">{formatNimPrice(event.price_nim)} {event.currency}</span>;
}

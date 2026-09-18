import { CalendarDays } from "lucide-react";
import Link from "next/link";

import { ExternalMediaImage } from "@/components/media/external-media-image";
import type { CalendarRecord } from "@/lib/api/calendars";

export function CalendarCard({ calendar }: { calendar: CalendarRecord }) {
  const archived = calendar.status === "archived";
  const fallback = <div className="grid size-full place-items-center bg-primary/15 text-accent" aria-hidden="true"><CalendarDays size={24} /></div>;
  const body = (
    <>
      <div className="h-28"><ExternalMediaImage src={calendar.image_url} alt="" className="size-full object-cover" fallback={fallback} /></div>
      <div className="space-y-2 p-4">
        <p className="text-base font-semibold text-foreground">{calendar.name}</p>
        {archived ? <p className="text-[11px] font-medium uppercase tracking-[0.14em] text-muted">Archived</p> : null}
        {calendar.description ? <p className="line-clamp-2 text-xs leading-5 text-muted">{calendar.description}</p> : null}
      </div>
    </>
  );
  if (archived) {
    return <div className="overflow-hidden rounded-xl border border-border bg-surface">{body}</div>;
  }
  return (
    <Link href={`/calendars/${calendar.id}`} className="group overflow-hidden rounded-xl border border-border bg-surface transition-colors hover:bg-surface-hover focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring">
      {body}
    </Link>
  );
}

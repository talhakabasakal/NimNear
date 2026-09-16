import { CalendarDays } from "lucide-react";
import Link from "next/link";

import { ExternalMediaImage } from "@/components/media/external-media-image";
import type { CalendarRecord } from "@/lib/api/calendars";

export function CalendarCard({ calendar }: { calendar: CalendarRecord }) {
  const fallback = <div className="grid size-full place-items-center bg-primary/15 text-accent" aria-hidden="true"><CalendarDays size={24} /></div>;
  return (
    <Link href={`/calendars/${calendar.id}`} className="group overflow-hidden rounded-xl border border-border bg-surface transition-colors hover:bg-surface-hover">
      <div className="h-28"><ExternalMediaImage src={calendar.image_url} alt="" className="size-full object-cover" fallback={fallback} /></div>
      <div className="space-y-2 p-4">
        <p className="text-base font-semibold text-foreground">{calendar.name}</p>
        {calendar.description ? <p className="line-clamp-2 text-xs leading-5 text-muted">{calendar.description}</p> : null}
      </div>
    </Link>
  );
}

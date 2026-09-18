import type { EventRecord } from "@/lib/api/events";
import { resolveCollectionState } from "@/lib/collection-state";

import { StateCard } from "@/components/app/state-card";

import { EventCard } from "./event-card";

type EventTimelineProps = {
  events: EventRecord[];
  error?: string;
};

const dateFormatter = new Intl.DateTimeFormat("en-US", { weekday: "long", day: "numeric", month: "long" });

export function EventTimeline({ events, error }: EventTimelineProps) {
  const state = resolveCollectionState(events, error);
  if (state.kind === "error") return <StateCard kind="error" title="Events could not be loaded" description="The event service is currently unavailable." />;
  if (state.kind === "empty") return <StateCard kind="empty" title="No events during this period" description="Try again by selecting a different time range." />;

  const groups = state.records.reduce<Record<string, EventRecord[]>>((result, event) => {
    const key = new Date(event.starts_at).toISOString().slice(0, 10);
    result[key] ??= [];
    result[key].push(event);
    return result;
  }, {});

  return (
    <div className="space-y-8">
      {Object.entries(groups).map(([date, group]) => (
        <section key={date} className="grid gap-4 md:grid-cols-[150px_1fr] md:gap-8">
          <div className="flex items-start gap-3 md:block">
            <span className="mt-1 size-2 shrink-0 rounded-full bg-primary md:mb-3 md:block" />
            <h2 className="text-sm font-semibold capitalize text-foreground">{dateFormatter.format(new Date(date + "T12:00:00"))}</h2>
          </div>
          <div className="grid gap-4 md:grid-cols-2">{group.map((event) => <EventCard key={event.id} event={event} />)}</div>
        </section>
      ))}
    </div>
  );
}

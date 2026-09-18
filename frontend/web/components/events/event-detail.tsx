"use client";

import { CalendarDays, Clock3, MapPin, UserRound, Users } from "lucide-react";
import Link from "next/link";
import { useCallback, useState } from "react";

import { formatNimPrice, type EventRecord } from "@/lib/api/events";
import type { ProfileRecord } from "@/lib/api/profiles";

import { EventDetailHero } from "./event-detail-hero";
import { ExternalMediaImage } from "@/components/media/external-media-image";
import { EventParticipation } from "./event-participation";
import { EventPurchase } from "./event-purchase";
import { EventStatusBadge } from "./event-status-badge";
import { EventOrganizerActions } from "./event-organizer-actions";

type EventDetailProps = {
  event: EventRecord;
  organizer?: ProfileRecord | null;
  organizerUnavailable?: boolean;
};

const dateFormatter = new Intl.DateTimeFormat("en-US", {
  weekday: "long",
  day: "numeric",
  month: "long",
  year: "numeric",
});
const timeFormatter = new Intl.DateTimeFormat("en-US", {
  hour: "2-digit",
  minute: "2-digit",
});

function formatDate(value: string) {
  const date = new Date(value);
  return Number.isNaN(date.getTime())
    ? "Date unavailable"
    : dateFormatter.format(date);
}

function formatTime(value: string) {
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? "--:--" : timeFormatter.format(date);
}

function formatAttendance(capacity: number | null, attendeeCount: number) {
  return capacity === null
    ? String(attendeeCount) + " attendees"
    : String(attendeeCount) + " / " + String(capacity) + " attendees";
}

export function EventDetail({
  event,
  organizer,
  organizerUnavailable = false,
}: EventDetailProps) {
  const [attendance, setAttendance] = useState({
    attendeeCount: event.attendee_count,
    isSoldOut: event.is_sold_out,
  });
  const updateAttendance = useCallback(
    (state: { attendee_count: number; is_sold_out: boolean }) => {
      setAttendance({
        attendeeCount: state.attendee_count,
        isSoldOut: state.is_sold_out,
      });
    },
    [],
  );

  return (
    <article className="space-y-6">
      <EventDetailHero imageUrl={event.image_url} title={event.title} />

      {event.status === "cancelled" ? (
        <section
          className="rounded-xl border border-red-300/30 bg-red-400/10 p-5"
          role="status"
        >
          <p className="text-xs font-medium uppercase tracking-[0.16em] text-red-200">
            Event cancelled
          </p>
          <p className="mt-2 text-sm leading-6 text-muted">
            This event is preserved for reference. New attendance and payment
            actions are unavailable. Refunds are not implemented.
          </p>
        </section>
      ) : null}

      <div className="grid gap-6 lg:grid-cols-[minmax(0,1fr)_320px] lg:items-start">
        <div className="space-y-6">
          <header className="space-y-4">
            <div className="flex flex-wrap items-center gap-2">
              <span className="text-xs font-medium uppercase tracking-[0.16em] text-accent">
                {event.city}
              </span>
              <EventStatusBadge event={event} />
            </div>
            <h1 className="max-w-3xl text-3xl font-semibold leading-tight tracking-[-0.035em] text-foreground sm:text-[42px]">
              {event.title}
            </h1>
          </header>

          {event.description ? (
            <section className="rounded-xl border border-border bg-surface p-5 sm:p-6">
              <h2 className="text-sm font-semibold text-foreground">
                About the event
              </h2>
              <p className="mt-3 whitespace-pre-wrap text-sm leading-6 text-muted">
                {event.description}
              </p>
            </section>
          ) : null}
        </div>

        <aside className="space-y-3">
          <section className="space-y-4 rounded-xl border border-border bg-surface p-5">
            <div className="flex gap-3">
              <CalendarDays size={18} className="mt-0.5 shrink-0 text-accent" />
              <div>
                <p className="text-xs text-muted">Date</p>
                <p className="mt-1 text-sm font-medium capitalize text-foreground">
                  {formatDate(event.starts_at)}
                </p>
              </div>
            </div>
            <div className="flex gap-3">
              <Clock3 size={18} className="mt-0.5 shrink-0 text-accent" />
              <div>
                <p className="text-xs text-muted">Time</p>
                <p className="mt-1 text-sm font-medium text-foreground">
                  {formatTime(event.starts_at)} – {formatTime(event.ends_at)}
                </p>
              </div>
            </div>
            <div className="flex gap-3">
              <MapPin size={18} className="mt-0.5 shrink-0 text-accent" />
              <div>
                <p className="text-xs text-muted">Location</p>
                <p className="mt-1 text-sm font-medium text-foreground">
                  {event.address || event.city}
                </p>
                {event.address ? (
                  <p className="mt-1 text-xs text-muted">{event.city}</p>
                ) : null}
              </div>
            </div>
            <div className="flex gap-3">
              <Users size={18} className="mt-0.5 shrink-0 text-accent" />
              <div>
                <p className="text-xs text-muted">Attendance</p>
                <p className="mt-1 text-sm font-medium text-foreground">
                  {formatAttendance(event.capacity, attendance.attendeeCount)}
                </p>
              </div>
            </div>
            <div className="border-t border-border pt-4">
              <p className="text-xs text-muted">Ticket price</p>
              <p className="mt-1 text-lg font-semibold text-foreground">
                {event.is_free
                  ? "Free"
                  : `${formatNimPrice(event.price_nim)} ${event.currency}`}
              </p>
            </div>
          </section>
          {event.status !== "cancelled" ? (
            <EventParticipation
              eventId={event.id}
              isFree={event.is_free}
              isPast={event.is_past}
              isSoldOut={attendance.isSoldOut}
              attendeeCount={attendance.attendeeCount}
              capacity={event.capacity}
              onStateChange={updateAttendance}
            />
          ) : null}
          {event.status !== "cancelled" && !event.is_free ? (
            <EventPurchase
              eventId={event.id}
              isPast={event.is_past}
              isSoldOut={attendance.isSoldOut}
            />
          ) : null}
          {event.status !== "cancelled" ? (
            <EventOrganizerActions event={event} />
          ) : null}
          {organizer ? (
            <Link
              href={"/profiles/" + organizer.id}
              className="flex items-center gap-3 rounded-xl border border-border bg-surface p-4 transition-colors hover:bg-surface-hover"
            >
              <ExternalMediaImage
                src={organizer.avatar_url}
                className="size-10 rounded-full border border-border object-cover"
                fallback={
                  <span className="grid size-10 place-items-center rounded-full bg-avatar text-white">
                    <UserRound size={18} />
                  </span>
                }
              />
              <span className="min-w-0">
                <span className="block text-xs text-muted">Organizer</span>
                <span className="mt-0.5 block truncate text-sm font-medium text-foreground">
                  {organizer.display_name.trim() ||
                    organizer.wallet_address?.trim() ||
                    "NIMNear user"}
                </span>
                {organizer.wallet_address ? (
                  <span className="mt-0.5 block truncate font-mono text-xs text-muted">
                    {organizer.wallet_address}
                  </span>
                ) : null}
                {organizer.username ? (
                  <span className="block text-xs text-muted">
                    @{organizer.username}
                  </span>
                ) : null}
              </span>
            </Link>
          ) : null}
          {!organizer && organizerUnavailable ? (
            <section className="rounded-xl border border-border bg-surface p-4">
              <p className="text-xs font-medium text-foreground">
                Organizer information could not be loaded
              </p>
              <p className="mt-1 text-xs leading-5 text-muted">
                The profile service is currently unavailable.
              </p>
            </section>
          ) : null}
        </aside>
      </div>
    </article>
  );
}

"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";

import { Button } from "@/components/ui/button";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import {
  AuthApiError,
  fetchCurrentUser,
  readAuthSession,
} from "@/lib/api/auth";
import { cancelEvent, updateEvent, type EventRecord } from "@/lib/api/events";
import { userFacingCaughtError } from "@/lib/api/http-error";

type Props = { event: EventRecord };
type FormState = {
  title: string;
  description: string;
  startsAt: string;
  endsAt: string;
  imageURL: string;
  capacity: string;
};

function localDateTime(value: string) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "";
  const pad = (part: number) => String(part).padStart(2, "0");
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}T${pad(date.getHours())}:${pad(date.getMinutes())}`;
}

function toISO(value: string) {
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? null : date.toISOString();
}

export function EventOrganizerActions({ event }: Props) {
  const router = useRouter();
  const [allowed, setAllowed] = useState(false);
  const [editing, setEditing] = useState(false);
  const [pending, setPending] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [confirmCancel, setConfirmCancel] = useState(false);
  const [values, setValues] = useState<FormState>(() => ({
    title: event.title,
    description: event.description,
    startsAt: localDateTime(event.starts_at),
    endsAt: localDateTime(event.ends_at),
    imageURL: event.image_url ?? "",
    capacity: event.capacity === null ? "" : String(event.capacity),
  }));

  useEffect(() => {
    const stored = readAuthSession();
    if (!stored || !event.organizer_id) return;
    fetchCurrentUser()
      .then((user) => setAllowed(user.id === event.organizer_id))
      .catch((requestError: unknown) => {
        if (!(requestError instanceof AuthApiError))
          setError("Organizer controls could not be loaded.");
      });
  }, [event.organizer_id]);

  if (!allowed || event.status === "cancelled") return null;

  function setValue(key: keyof FormState, value: string) {
    setValues((current) => ({ ...current, [key]: value }));
    setError(null);
  }

  async function save() {
    if (pending) return;
    const startsAt = toISO(values.startsAt);
    const endsAt = toISO(values.endsAt);
    if (
      !values.title.trim() ||
      !startsAt ||
      !endsAt ||
      new Date(endsAt) <= new Date(startsAt)
    ) {
      setError("Title and a valid start/end time are required.");
      return;
    }
    const capacity = values.capacity.trim() ? Number(values.capacity) : null;
    if (
      capacity !== null &&
      (!Number.isSafeInteger(capacity) || capacity < 1)
    ) {
      setError("Capacity must be a positive integer.");
      return;
    }
    setPending(true);
    setError(null);
    try {
      await updateEvent(event.id, {
        title: values.title.trim(),
        description: values.description,
        starts_at: startsAt,
        ends_at: endsAt,
        image_url: values.imageURL.trim(),
        capacity,
      });
      setEditing(false);
      router.refresh();
    } catch (requestError) {
      setError(userFacingCaughtError(requestError, "The event could not be updated."));
    } finally {
      setPending(false);
    }
  }

  async function cancel() {
    if (pending) return;
    setPending(true);
    setError(null);
    try {
      await cancelEvent(event.id);
      setConfirmCancel(false);
      router.refresh();
    } catch (requestError) {
      setError(userFacingCaughtError(requestError, "The event could not be cancelled."));
    } finally {
      setPending(false);
    }
  }

  return (
    <section className="rounded-xl border border-border bg-surface p-5">
      <p className="text-xs font-medium uppercase tracking-[0.16em] text-accent">
        Organizer controls
      </p>
      {!editing ? (
        <div className="mt-3 grid gap-2 sm:grid-cols-2">
          <Button
            type="button"
            variant="outline"
            onClick={() => setEditing(true)}
            disabled={pending}
          >
            Edit event
          </Button>
          <Button
            type="button"
            variant="outline"
            onClick={() => setConfirmCancel(true)}
            disabled={pending}
            className="border-red-300/30 text-red-200 hover:bg-red-400/10"
          >
            Cancel event
          </Button>
        </div>
      ) : (
        <form
          className="mt-4 space-y-3"
          onSubmit={(event) => {
            event.preventDefault();
            void save();
          }}
        >
          <label className="block text-xs text-muted">
            Title
            <input
              value={values.title}
              onChange={(event) => setValue("title", event.target.value)}
              className="mt-1 h-10 w-full rounded-lg border border-border bg-background px-3 text-sm text-foreground"
            />
          </label>
          <label className="block text-xs text-muted">
            Description
            <textarea
              value={values.description}
              onChange={(event) => setValue("description", event.target.value)}
              rows={4}
              className="mt-1 w-full rounded-lg border border-border bg-background px-3 py-2 text-sm text-foreground"
            />
          </label>
          <div className="grid gap-3 sm:grid-cols-2">
            <label className="block text-xs text-muted">
              Starts
              <input
                type="datetime-local"
                value={values.startsAt}
                onChange={(event) => setValue("startsAt", event.target.value)}
                className="mt-1 h-10 w-full rounded-lg border border-border bg-background px-2 text-sm text-foreground"
              />
            </label>
            <label className="block text-xs text-muted">
              Ends
              <input
                type="datetime-local"
                value={values.endsAt}
                onChange={(event) => setValue("endsAt", event.target.value)}
                className="mt-1 h-10 w-full rounded-lg border border-border bg-background px-2 text-sm text-foreground"
              />
            </label>
          </div>
          <div className="grid gap-3 sm:grid-cols-2">
            <label className="block text-xs text-muted">
              Image URL
              <input
                value={values.imageURL}
                onChange={(event) => setValue("imageURL", event.target.value)}
                className="mt-1 h-10 w-full rounded-lg border border-border bg-background px-3 text-sm text-foreground"
              />
            </label>
            <label className="block text-xs text-muted">
              Capacity
              <input
                inputMode="numeric"
                value={values.capacity}
                onChange={(event) => setValue("capacity", event.target.value)}
                placeholder="Unlimited"
                className="mt-1 h-10 w-full rounded-lg border border-border bg-background px-3 text-sm text-foreground"
              />
            </label>
          </div>
          <div className="flex gap-2">
            <Button type="submit" disabled={pending}>
              {pending ? "Saving…" : "Save changes"}
            </Button>
            <Button
              type="button"
              variant="outline"
              onClick={() => setEditing(false)}
              disabled={pending}
            >
              Close
            </Button>
          </div>
        </form>
      )}
      {error ? (
        <p className="mt-3 text-xs leading-5 text-red-200" role="alert">{error}</p>
      ) : null}
      <ConfirmDialog
        open={confirmCancel}
        title="Cancel this event?"
        description="Existing RSVP and payment evidence will be preserved. Refunds are not handled here."
        confirmLabel="Cancel event"
        pending={pending}
        pendingLabel="Cancelling…"
        destructive
        onConfirm={() => {
          void cancel();
        }}
        onOpenChange={setConfirmCancel}
      />
    </section>
  );
}

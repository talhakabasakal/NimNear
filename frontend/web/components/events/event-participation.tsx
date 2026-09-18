"use client";

import { useEffect, useState } from "react";

import { NimiqConnect } from "@/components/auth/nimiq-connect";
import { Button } from "@/components/ui/button";
import {
  AuthApiError,
  clearAuthSession,
  fetchCurrentUser,
  isLostSessionStatus,
  readAuthSession,
  type AuthSession,
  writeAuthSession,
} from "@/lib/api/auth";
import {
  cancelParticipation,
  createParticipation,
  fetchParticipation,
  ParticipationApiError,
  type ParticipationState,
} from "@/lib/api/participation";

type EventParticipationProps = {
  eventId: string;
  isFree: boolean;
  isPast: boolean;
  isSoldOut: boolean;
  attendeeCount: number;
  capacity: number | null;
  onStateChange?: (state: ParticipationState) => void;
};

type LoadStatus = "checking" | "anonymous" | "authenticated" | "error";

export function EventParticipation({
  eventId,
  isFree,
  isPast,
  isSoldOut,
  attendeeCount,
  capacity,
  onStateChange,
}: EventParticipationProps) {
  const [status, setStatus] = useState<LoadStatus>("checking");
  const [session, setSession] = useState<AuthSession | null>(null);
  const [participation, setParticipation] = useState<ParticipationState>({
    event_id: eventId,
    attending: false,
    attendee_count: attendeeCount,
    capacity,
    is_sold_out: isSoldOut,
  });
  const [pending, setPending] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [retryKey, setRetryKey] = useState(0);

  useEffect(() => {
    if (!isFree || isPast) {
      setStatus("anonymous");
      return;
    }

    const stored = readAuthSession();
    if (!stored) {
      setStatus("anonymous");
      return;
    }

    const storedSession = stored;
    let active = true;
    async function restore() {
      try {
        const user = await fetchCurrentUser();
        const nextParticipation = await fetchParticipation(eventId);
        if (!active) return;
        const refreshed = { ...storedSession, user };
        writeAuthSession(refreshed);
        setSession(refreshed);
        setParticipation(nextParticipation);
        onStateChange?.(nextParticipation);
        setStatus("authenticated");
      } catch (requestError) {
        if (!active) return;
        const isUnauthorized =
          (requestError instanceof AuthApiError && isLostSessionStatus(requestError.status))
          || (requestError instanceof ParticipationApiError && requestError.status === 401);
        if (isUnauthorized) {
          clearAuthSession();
          setSession(null);
          setStatus("anonymous");
          setError("Your session has expired. Sign in again with your Nimiq wallet.");
          return;
        }
        setError(requestError instanceof Error ? requestError.message : "Attendance status could not be retrieved.");
        setStatus("error");
      }
    }
    void restore();
    return () => {
      active = false;
    };
  }, [eventId, isFree, isPast, onStateChange, retryKey]);

  if (!isFree) return null;

  if (isPast) {
    return null;
  }

  if (status === "checking") {
    return <div className="h-32 animate-pulse rounded-xl border border-border bg-surface" aria-label="Checking attendance status" />;
  }

  if (status === "error") {
    return (
      <section className="rounded-xl border border-red-300/20 bg-red-400/10 p-5">
        <p className="text-xs font-medium uppercase tracking-[0.16em] text-red-200">Attendance</p>
        <p className="mt-2 text-sm font-medium text-foreground">Attendance status could not be loaded.</p>
        <p className="mt-1 text-xs leading-5 text-muted">{error ?? "The event service is currently unavailable."}</p>
        <Button type="button" variant="outline" className="mt-4 w-full" onClick={() => { setError(null); setStatus("checking"); setRetryKey((value) => value + 1); }}>Try again</Button>
      </section>
    );
  }

  if (status === "anonymous") {
    if (participation.is_sold_out) {
      return <Button type="button" variant="outline" disabled className="w-full">Sold out</Button>;
    }
    return (
      <NimiqConnect description="Sign in with your Nimiq wallet to attend the event." />
    );
  }

  async function handleParticipation() {
    if (!session || pending) return;
    setPending(true);
    setError(null);
    try {
      const nextParticipation = participation.attending
        ? await cancelParticipation(eventId)
        : await createParticipation(eventId);
      setParticipation(nextParticipation);
      onStateChange?.(nextParticipation);
    } catch (requestError) {
      if (requestError instanceof ParticipationApiError && requestError.status === 401) {
        clearAuthSession();
        setSession(null);
        setStatus("anonymous");
        setError("Your session has expired. Sign in again with your Nimiq wallet.");
      } else {
        setError(requestError instanceof ParticipationApiError ? requestError.message : "Attendance action could not be completed.");
      }
    } finally {
      setPending(false);
    }
  }

  const canJoin = !participation.is_sold_out || participation.attending;
  const attendanceLabel = capacity === null
    ? "Unlimited capacity."
    : participation.attendee_count + " / " + capacity + " people are attending.";

  return (
    <section className="rounded-xl border border-border bg-surface p-5">
      <p className="text-xs font-medium uppercase tracking-[0.16em] text-accent">Attendance</p>
      <p className="mt-2 text-sm leading-6 text-muted">
        {participation.attending ? "You are attending this event." : attendanceLabel}
      </p>
      <Button type="button" variant={participation.attending ? "outline" : "default"} disabled={pending || !canJoin} onClick={handleParticipation} className="mt-4 w-full">
        {pending ? "Waiting…" : participation.attending ? "Cancel attendance" : participation.is_sold_out ? "Sold out" : "Join"}
      </Button>
      {error ? <p className="mt-3 text-xs leading-5 text-red-200">{error}</p> : null}
    </section>
  );
}

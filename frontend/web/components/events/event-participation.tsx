"use client";

import { useEffect, useState } from "react";

import { AuthPanel } from "@/components/auth/auth-panel";
import { Button } from "@/components/ui/button";
import {
  clearAuthSession,
  fetchCurrentUser,
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

type LoadStatus = "checking" | "anonymous" | "authenticated";

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
        const user = await fetchCurrentUser(storedSession.token);
        const nextParticipation = await fetchParticipation(eventId, storedSession.token);
        if (!active) return;
        const refreshed = { ...storedSession, user };
        writeAuthSession(refreshed);
        setSession(refreshed);
        setParticipation(nextParticipation);
        onStateChange?.(nextParticipation);
        setStatus("authenticated");
      } catch {
        if (!active) return;
        clearAuthSession();
        setSession(null);
        setStatus("anonymous");
      }
    }
    void restore();
    return () => {
      active = false;
    };
  }, [eventId, isFree, isPast, onStateChange]);

  if (!isFree) return null;

  if (isPast) {
    return null;
  }

  async function refreshParticipation(nextSession: AuthSession) {
    setSession(nextSession);
    setStatus("authenticated");
    setError(null);
    try {
      const nextParticipation = await fetchParticipation(eventId, nextSession.token);
      setParticipation(nextParticipation);
      onStateChange?.(nextParticipation);
    } catch (requestError) {
      if (requestError instanceof ParticipationApiError && requestError.status === 401) {
        clearAuthSession();
        setSession(null);
        setStatus("anonymous");
        setError("Oturumun sona ermiş. Tekrar giriş yapmalısın.");
      } else {
        setError(requestError instanceof ParticipationApiError ? requestError.message : "Katılım durumu alınamadı.");
      }
    }
  }

  if (status === "checking") {
    return <div className="h-32 animate-pulse rounded-xl border border-border bg-surface" aria-label="Katılım durumu kontrol ediliyor" />;
  }

  if (status === "anonymous") {
    if (participation.is_sold_out) {
      return <Button type="button" variant="outline" disabled className="w-full">Tükendi</Button>;
    }
    return (
      <AuthPanel
        onAuthenticated={(nextSession) => { void refreshParticipation(nextSession); }}
        title="Katılmak için giriş yap"
        description="Ücretsiz etkinliklere katılmak için hesabınla devam et."
      />
    );
  }

  async function handleParticipation() {
    if (!session || pending) return;
    setPending(true);
    setError(null);
    try {
      const nextParticipation = participation.attending
        ? await cancelParticipation(eventId, session.token)
        : await createParticipation(eventId, session.token);
      setParticipation(nextParticipation);
      onStateChange?.(nextParticipation);
    } catch (requestError) {
      if (requestError instanceof ParticipationApiError && requestError.status === 401) {
        clearAuthSession();
        setSession(null);
        setStatus("anonymous");
        setError("Oturumun sona ermiş. Tekrar giriş yapmalısın.");
      } else {
        setError(requestError instanceof ParticipationApiError ? requestError.message : "Katılım işlemi tamamlanamadı.");
      }
    } finally {
      setPending(false);
    }
  }

  const canJoin = !participation.is_sold_out || participation.attending;
  const attendanceLabel = capacity === null
    ? "Kontenjan sınırsız."
    : participation.attendee_count + " / " + capacity + " kişi katılıyor.";

  return (
    <section className="rounded-xl border border-border bg-surface p-5">
      <p className="text-xs font-medium uppercase tracking-[0.16em] text-accent">Katılım</p>
      <p className="mt-2 text-sm leading-6 text-muted">
        {participation.attending ? "Bu etkinliğe katılıyorsun." : attendanceLabel}
      </p>
      <Button type="button" variant={participation.attending ? "outline" : "default"} disabled={pending || !canJoin} onClick={handleParticipation} className="mt-4 w-full">
        {pending ? "Bekleniyor…" : participation.attending ? "Katılımı iptal et" : participation.is_sold_out ? "Tükendi" : "Katıl"}
      </Button>
      {error ? <p className="mt-3 text-xs leading-5 text-red-200">{error}</p> : null}
    </section>
  );
}

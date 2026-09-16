"use client";

import { useState } from "react";

import { Button } from "@/components/ui/button";
import { followCalendar, unfollowCalendar } from "@/lib/api/calendars";
import { readAuthSession } from "@/lib/api/auth";

export function CalendarFollowButton({ calendarId, initialFollowing = false }: { calendarId: string; initialFollowing?: boolean }) {
  const [following, setFollowing] = useState(initialFollowing);
  const [pending, setPending] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function toggle() {
    if (pending) return;
    const session = readAuthSession();
    if (!session) {
      setError("Takip etmek için NIMNear backend oturumu gerekli. Nimiq bağlantısı tek başına yetkilendirme sağlamaz.");
      return;
    }
    setPending(true);
    setError(null);
    try {
      if (following) await unfollowCalendar(calendarId, session.token);
      else await followCalendar(calendarId, session.token);
      setFollowing((value) => !value);
    } catch {
      setError("Takvim takip durumu güncellenemedi.");
    } finally {
      setPending(false);
    }
  }

  return (
    <div className="space-y-2">
      <Button type="button" variant={following ? "soft" : "outline"} size="sm" disabled={pending} onClick={() => { void toggle(); }}>
        {pending ? "Güncelleniyor…" : following ? "Takipte" : "Takip et"}
      </Button>
      {error ? <p className="max-w-xs text-right text-[11px] leading-4 text-red-200" role="alert">{error}</p> : null}
    </div>
  );
}

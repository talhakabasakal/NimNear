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
      setError("A NIMNear backend session is required to follow calendars. A Nimiq connection alone does not provide authorization.");
      return;
    }
    setPending(true);
    setError(null);
    try {
      if (following) await unfollowCalendar(calendarId);
      else await followCalendar(calendarId);
      setFollowing((value) => !value);
    } catch {
      setError("Calendar follow status could not be updated.");
    } finally {
      setPending(false);
    }
  }

  return (
    <div className="space-y-2">
      <Button type="button" variant={following ? "soft" : "outline"} size="sm" disabled={pending} onClick={() => { void toggle(); }}>
        {pending ? "Updating…" : following ? "Following" : "Follow"}
      </Button>
      {error ? <p className="max-w-xs text-right text-[11px] leading-4 text-red-200" role="alert">{error}</p> : null}
    </div>
  );
}

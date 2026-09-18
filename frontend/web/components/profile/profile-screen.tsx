"use client";

import { CalendarDays, UserRound } from "lucide-react";
import Link from "next/link";
import { useEffect, useMemo, useState } from "react";
import { ExternalMediaImage } from "@/components/media/external-media-image";

import { AppHeader } from "@/components/app/app-header";
import { StateCard } from "@/components/app/state-card";
import { AccountProfile } from "@/components/profile/account-profile";
import { NimiqIdenticon } from "@/components/profile/nimiq-identicon";
import { EventCard } from "@/components/events/event-card";
import { fetchProfile, fetchProfileEvents, ProfilesApiError, type ProfileRecord } from "@/lib/api/profiles";
import { NimiqConnect } from "@/components/auth/nimiq-connect";
import { clearAuthSession, isLostSessionStatus, readAuthSession, type AuthSession } from "@/lib/api/auth";
import type { EventRecord } from "@/lib/api/events";
import { compactNimiqAddress, shortenNimiqAddress } from "@/lib/nimiq/address";

type ProfileScreenProps = {
  profileId?: string;
  initialProfile?: ProfileRecord;
};

const joinedFormatter = new Intl.DateTimeFormat("en-US", { month: "long", year: "numeric" });

function profileName(profile: ProfileRecord) {
  return (
    profile.display_name.trim() ||
    (profile.wallet_address ? shortenNimiqAddress(profile.wallet_address) : "") ||
    "NIMNear user"
  );
}

function ProfileAvatar({ profile }: { profile: ProfileRecord }) {
  const walletSeed = profile.wallet_address ? compactNimiqAddress(profile.wallet_address) : "";
  const identicon = walletSeed ? (
    <NimiqIdenticon seed={walletSeed} className="size-20 sm:size-24" alt="" />
  ) : (
    <span className="grid size-20 place-items-center rounded-full bg-avatar text-white sm:size-24">
      <UserRound size={30} />
    </span>
  );

  return (
    <ExternalMediaImage
      src={profile.avatar_url}
      className="size-20 rounded-full border border-border object-cover sm:size-24"
      fallback={identicon}
    />
  );
}

function EventSection({ title, events, emptyDescription }: { title: string; events: EventRecord[]; emptyDescription: string }) {
  return (
    <section className="space-y-4">
      <div>
        <p className="text-xs font-medium uppercase tracking-[0.16em] text-accent">Events</p>
        <h2 className="mt-1 text-xl font-semibold tracking-[-0.025em] text-foreground">{title}</h2>
      </div>
      {events.length === 0 ? <StateCard kind="empty" title="No events yet" description={emptyDescription} /> : <div className="grid gap-4 md:grid-cols-2">{events.map((event) => <EventCard key={event.id} event={event} compact />)}</div>}
    </section>
  );
}

function PublicProfileContent({ profile, organized, attended }: { profile: ProfileRecord; organized: EventRecord[]; attended: EventRecord[] }) {
  const [activeTab, setActiveTab] = useState<"organized" | "attended">("organized");
  const activeEvents = activeTab === "organized" ? organized : attended;
  const activeTitle = activeTab === "organized" ? "Events organized" : "Events attended";
  const activeDescription = activeTab === "organized" ? "This user has not organized a public event yet." : "This user has not attended an event yet.";
  const joinedAt = useMemo(() => {
    const date = new Date(profile.joined_at);
    return Number.isNaN(date.getTime()) ? "Date unavailable" : joinedFormatter.format(date);
  }, [profile.joined_at]);

  return (
    <main className="mx-auto max-w-[1000px] space-y-8 px-4 py-8 sm:px-6 sm:py-10 lg:px-8">
      <section className="rounded-xl border border-border bg-surface p-5 sm:p-7">
        <div className="flex flex-col gap-5 sm:flex-row sm:items-center">
          <ProfileAvatar profile={profile} />
          <div className="min-w-0">
            <p className="text-xs font-medium uppercase tracking-[0.16em] text-accent">Profile</p>
            <h1 className="mt-1 truncate text-2xl font-semibold tracking-[-0.03em] text-foreground sm:text-[28px]">{profileName(profile)}</h1>
            {profile.wallet_address ? <p className="mt-1 break-all font-mono text-sm text-muted">{profile.wallet_address}</p> : null}
            {profile.username ? <p className="mt-1 text-sm text-muted">@{profile.username}</p> : null}
            {profile.bio ? <p className="mt-3 max-w-xl whitespace-pre-wrap text-sm leading-6 text-muted">{profile.bio}</p> : null}
            <p className="mt-3 flex items-center gap-2 text-xs text-muted"><CalendarDays size={14} className="text-accent" /> Joined NIMNear on {joinedAt}</p>
          </div>
        </div>
        <div className="mt-6 grid max-w-md grid-cols-2 gap-3 border-t border-border pt-5">
          <div><p className="text-2xl font-semibold text-foreground">{profile.organized_event_count}</p><p className="mt-1 text-xs text-muted">Events organized</p></div>
          <div><p className="text-2xl font-semibold text-foreground">{profile.attended_event_count}</p><p className="mt-1 text-xs text-muted">Events attended</p></div>
        </div>
      </section>

      <div
        className="flex w-fit rounded-lg border border-border bg-surface p-1"
        role="tablist"
        aria-label="Profile events"
        onKeyDown={(event) => {
          if (event.key === "ArrowRight" || event.key === "ArrowLeft") {
            event.preventDefault();
            setActiveTab((current) => (current === "organized" ? "attended" : "organized"));
          }
        }}
      >
        <button type="button" role="tab" id="organized-tab" aria-controls="profile-events-panel" aria-selected={activeTab === "organized"} onClick={() => setActiveTab("organized")} className={"rounded-md px-3 py-2 text-xs font-medium " + (activeTab === "organized" ? "bg-surface-hover text-foreground" : "text-muted")}>Organized</button>
        <button type="button" role="tab" id="attended-tab" aria-controls="profile-events-panel" aria-selected={activeTab === "attended"} onClick={() => setActiveTab("attended")} className={"rounded-md px-3 py-2 text-xs font-medium " + (activeTab === "attended" ? "bg-surface-hover text-foreground" : "text-muted")}>Attended</button>
      </div>

      <div id="profile-events-panel" role="tabpanel" aria-labelledby={activeTab === "organized" ? "organized-tab" : "attended-tab"}>
        <EventSection title={activeTitle} events={activeEvents} emptyDescription={activeDescription} />
      </div>
    </main>
  );
}

function OwnProfileContent({
  profile,
  organized,
  attended,
  session,
  onProfileChange,
}: {
  profile: ProfileRecord;
  organized: EventRecord[];
  attended: EventRecord[];
  session: AuthSession;
  onProfileChange: (profile: ProfileRecord) => void;
}) {
  const [activeTab, setActiveTab] = useState<"organized" | "attended">("organized");
  const activeEvents = activeTab === "organized" ? organized : attended;
  const activeTitle = activeTab === "organized" ? "Events organized" : "Events attended";
  const activeDescription = activeTab === "organized" ? "You have not organized a public event yet." : "You have not attended an event yet.";

  return (
    <main className="mx-auto max-w-[800px] space-y-10 px-4 py-8 sm:px-6 sm:py-10 lg:px-8">
      <AccountProfile profile={profile} session={session} onProfileChange={onProfileChange} />

      <div
        className="flex w-fit rounded-lg border border-border bg-surface p-1"
        role="tablist"
        aria-label="Profile events"
        onKeyDown={(event) => {
          if (event.key === "ArrowRight" || event.key === "ArrowLeft") {
            event.preventDefault();
            setActiveTab((current) => (current === "organized" ? "attended" : "organized"));
          }
        }}
      >
        <button type="button" role="tab" id="organized-tab" aria-controls="profile-events-panel" aria-selected={activeTab === "organized"} onClick={() => setActiveTab("organized")} className={"rounded-md px-3 py-2 text-xs font-medium " + (activeTab === "organized" ? "bg-surface-hover text-foreground" : "text-muted")}>Organized</button>
        <button type="button" role="tab" id="attended-tab" aria-controls="profile-events-panel" aria-selected={activeTab === "attended"} onClick={() => setActiveTab("attended")} className={"rounded-md px-3 py-2 text-xs font-medium " + (activeTab === "attended" ? "bg-surface-hover text-foreground" : "text-muted")}>Attended</button>
      </div>

      <div id="profile-events-panel" role="tabpanel" aria-labelledby={activeTab === "organized" ? "organized-tab" : "attended-tab"}>
        <EventSection title={activeTitle} events={activeEvents} emptyDescription={activeDescription} />
      </div>
      {activeTab === "organized" && organized.length === 0 ? (
        <Link href="/events/create" className="inline-flex h-10 items-center rounded-lg bg-primary px-4 text-sm font-medium text-white transition-colors hover:opacity-90">
          Create your first event
        </Link>
      ) : null}
    </main>
  );
}

export function ProfileScreen({ profileId, initialProfile }: ProfileScreenProps) {
  const [profile, setProfile] = useState<ProfileRecord | null>(initialProfile ?? null);
  const [organized, setOrganized] = useState<EventRecord[]>([]);
  const [attended, setAttended] = useState<EventRecord[]>([]);
  const [session, setSession] = useState<AuthSession | null>(null);
  const [loading, setLoading] = useState(!initialProfile);
  const [needsAuth, setNeedsAuth] = useState(false);
  const [error, setError] = useState(false);

  useEffect(() => {
    let cancelled = false;
    async function load() {
      const currentSession = readAuthSession();
      if (!profileId && !currentSession) {
        if (!cancelled) { setNeedsAuth(true); setLoading(false); }
        return;
      }
      const id = profileId ?? currentSession?.user.id;
      if (!id) return;

      try {
        const [loadedProfile, organizedEvents, attendedEvents] = await Promise.all([
          initialProfile ?? fetchProfile(id),
          fetchProfileEvents(id, "organized"),
          fetchProfileEvents(id, "attended"),
        ]);
        if (!cancelled) {
          setProfile(loadedProfile);
          setOrganized(organizedEvents);
          setAttended(attendedEvents);
          setSession(profileId ? null : currentSession);
        }
      } catch (error) {
        if (!cancelled && !profileId && currentSession && error instanceof ProfilesApiError && isLostSessionStatus(error.status)) {
          clearAuthSession();
          setNeedsAuth(true);
          return;
        }
        if (!cancelled) setError(true);
      } finally {
        if (!cancelled) setLoading(false);
      }
    }
    void load();
    return () => { cancelled = true; };
  }, [initialProfile, profileId]);

  if (needsAuth) {
    return <div className="min-h-svh bg-background"><AppHeader /><main className="mx-auto max-w-[760px] px-4 py-8 sm:px-6 sm:py-10 lg:px-8"><NimiqConnect description="Sign in securely with your Nimiq wallet." /></main></div>;
  }
  if (loading) {
    return <div className="min-h-svh bg-background"><AppHeader /><main className="mx-auto max-w-[800px] space-y-6 px-4 py-8 sm:px-6 sm:py-10 lg:px-8"><div className="h-40 animate-pulse rounded-xl border border-border bg-surface" /><div className="h-72 animate-pulse rounded-xl border border-border bg-surface" /></main></div>;
  }
  if (error || !profile) {
    return <div className="min-h-svh bg-background"><AppHeader /><main className="mx-auto max-w-[1000px] px-4 py-8 sm:px-6 sm:py-10 lg:px-8"><StateCard kind="error" title="Profile could not be loaded" description="The profile service is currently unavailable or the profile was not found." /></main></div>;
  }

  if (!profileId && session) {
    return (
      <div className="min-h-svh bg-background">
        <AppHeader />
        <OwnProfileContent
          profile={profile}
          organized={organized}
          attended={attended}
          session={session}
          onProfileChange={setProfile}
        />
      </div>
    );
  }

  return <div className="min-h-svh bg-background"><AppHeader /><PublicProfileContent profile={profile} organized={organized} attended={attended} /></div>;
}
